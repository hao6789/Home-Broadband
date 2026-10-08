package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// storeVersion 是 config.json 的 schema 版本。以后加配置项需要数据迁移时，
// 在这里加版本，并在 document.migrate 里写对应的迁移函数。
const storeVersion = 1

// storeFileName 是工作目录下统一存储的文件名。
const storeFileName = "config.json"

// document 是 config.json 的完整形态：配置与运行时状态收在一处，
// 0600 权限，原子写入（临时文件 + 改名）。
type document struct {
	// Version 是 schema 版本，migrate 时用。
	Version int `json:"version"`
	// Web 是管理界面的监听与功能开关。
	Web WebSettings `json:"web"`
	// BasePath 是访问路径前缀（形如 /xxx），空表示不加前缀。
	BasePath string `json:"basepath,omitempty"`
	// Password 是管理界面登录口令。
	Password string `json:"password,omitempty"`
	// PanelMode 是界面选过的后端（3x-ui/native），空表示自动探测。
	PanelMode string `json:"panel_mode,omitempty"`
	// XUIToken 是 3x-ui 面板 API 的 token。
	XUIToken string `json:"xui_token,omitempty"`
	// Sessions 是登录会话 token 到过期时间的映射。
	Sessions map[string]time.Time `json:"sessions,omitempty"`
	// Tunnels 是隧道状态的原始 JSON，core 包负责解释。
	Tunnels json.RawMessage `json:"tunnels,omitempty"`
}

// Store 是工作目录下唯一的持久化入口。
// 各域（web 设置、访问路径、口令、会话、面板模式、隧道状态）都经由它读写，
// 不再各自维护 0600 小文件。并发安全。
type Store struct {
	mu  sync.RWMutex
	dir string
	doc document
}

var (
	storesMu sync.Mutex
	stores   = map[string]*Store{}

	globalMu sync.RWMutex
	global   *Store
)

// Open 返回 dir 的统一存储（按目录缓存），首次打开时从老文件迁移。
// Open 是底层原语，不碰全局存储；需要"我现在在操作这个目录"语义时用 Init，
// 或直接调 LoadWebSettings / InitBasePath（它们会顺手把全局指向该目录）。
func Open(dir string) (*Store, error) {
	storesMu.Lock()
	defer storesMu.Unlock()
	if s, ok := stores[dir]; ok {
		return s, nil
	}
	s := &Store{dir: dir}
	if err := s.load(); err != nil {
		return nil, err
	}
	stores[dir] = s
	return s, nil
}

// Init 打开 dir 的存储并设为全局，供无 dir 参数的访问函数使用。
// main.go 启动时调一次；测试 helper 用临时目录调。
func Init(dir string) (*Store, error) {
	s, err := Open(dir)
	if err != nil {
		return nil, err
	}
	setGlobal(s)
	return s, nil
}

func setGlobal(s *Store) {
	globalMu.Lock()
	global = s
	globalMu.Unlock()
}

// globalStore 返回当前全局存储，可能为 nil（还没 Init 过）。
func globalStore() *Store {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return global
}

// load 从 config.json 读回；文件不存在时从老文件迁移；文件损坏时备份后重建。
func (s *Store) load() error {
	path := filepath.Join(s.dir, storeFileName)
	blob, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		return s.importLegacy()
	case err != nil:
		return fmt.Errorf("读 %s 失败: %w", storeFileName, err)
	}
	var doc document
	if err := json.Unmarshal(blob, &doc); err != nil {
		corrupt := path + ".corrupt"
		if rerr := os.Rename(path, corrupt); rerr != nil {
			log.Printf("配置 %s 损坏，但备份为 %s 失败: %v，用默认配置启动", path, corrupt, rerr)
		} else {
			log.Printf("配置 %s 损坏，已备份为 %s，用默认配置启动", path, corrupt)
		}
		return s.importLegacy()
	}
	if doc.Version > storeVersion {
		log.Printf("配置版本 %d 高于程序支持的 %d，尽量兼容读取", doc.Version, storeVersion)
	}
	if err := doc.migrate(); err != nil {
		return err
	}
	s.doc = doc
	return nil
}

// migrate 把老版本文档升到当前版本。
// 加新版本时在这里按版本号逐级迁移；降级（版本更高）不处理，尽量兼容读。
func (d *document) migrate() error {
	if d.Version < 1 {
		return fmt.Errorf("不支持的配置版本 %d", d.Version)
	}
	return nil
}

// importLegacy 把老版本散落的配置文件一次性导入统一存储。
// 严格模式：任一老文件损坏都直接报错，不静默丢配置；
// 成功后老文件改名 .bak（可手工恢复），新代码不再读它们。
func (s *Store) importLegacy() error {
	dir := s.dir
	doc := document{Version: storeVersion}
	var baks []string

	readLegacy := func(name string) ([]byte, bool, error) {
		blob, err := os.ReadFile(filepath.Join(dir, name))
		switch {
		case err == nil:
			return blob, true, nil
		case os.IsNotExist(err):
			return nil, false, nil
		default:
			return nil, false, fmt.Errorf("读 %s 失败: %w", name, err)
		}
	}
	corruptErr := func(name string, err error) error {
		return fmt.Errorf("迁移 %s 失败（文件损坏，请修复或删除后重试）: %w", name, err)
	}

	// settings.json
	if blob, ok, err := readLegacy("settings.json"); err != nil {
		return err
	} else if ok {
		var w WebSettings
		if err := json.Unmarshal(blob, &w); err != nil {
			return corruptErr("settings.json", err)
		}
		doc.Web = w
		baks = append(baks, "settings.json")
	}

	// basepath
	if blob, ok, err := readLegacy("basepath"); err != nil {
		return err
	} else if ok {
		if bp := strings.TrimSpace(string(blob)); bp != "" {
			doc.BasePath = normalizeBasePath(bp)
		}
		baks = append(baks, "basepath")
	}

	// password
	if blob, ok, err := readLegacy("password"); err != nil {
		return err
	} else if ok {
		doc.Password = strings.TrimSpace(string(blob))
		baks = append(baks, "password")
	}

	// sessions.json：只收未过期的
	if blob, ok, err := readLegacy("sessions.json"); err != nil {
		return err
	} else if ok {
		var saved map[string]time.Time
		if err := json.Unmarshal(blob, &saved); err != nil {
			return corruptErr("sessions.json", err)
		}
		now := time.Now()
		for tok, exp := range saved {
			if now.Before(exp) {
				if doc.Sessions == nil {
					doc.Sessions = map[string]time.Time{}
				}
				doc.Sessions[tok] = exp
			}
		}
		baks = append(baks, "sessions.json")
	}

	// state.json：core 负责解释，这里只收原始字节（先校验是合法 JSON）
	if blob, ok, err := readLegacy("state.json"); err != nil {
		return err
	} else if ok {
		var v any
		if err := json.Unmarshal(blob, &v); err != nil {
			return corruptErr("state.json", err)
		}
		doc.Tunnels = json.RawMessage(blob)
		baks = append(baks, "state.json")
	}

	// panel_mode
	if blob, ok, err := readLegacy("panel_mode"); err != nil {
		return err
	} else if ok {
		doc.PanelMode = strings.TrimSpace(string(blob))
		baks = append(baks, "panel_mode")
	}

	// xui-token
	if blob, ok, err := readLegacy("xui-token"); err != nil {
		return err
	} else if ok {
		doc.XUIToken = strings.TrimSpace(string(blob))
		baks = append(baks, "xui-token")
	}

	s.doc = doc
	if err := s.saveLocked(); err != nil {
		return err
	}
	for _, name := range baks {
		oldPath := filepath.Join(dir, name)
		if err := os.Rename(oldPath, oldPath+".bak"); err != nil {
			log.Printf("老配置文件 %s 改名 .bak 失败（不影响使用）: %v", oldPath, err)
		} else {
			log.Printf("已迁移 %s 到 %s，老文件改名为 %s.bak", name, storeFileName, name)
		}
	}
	return nil
}

// saveLocked 原子写盘（临时文件 + 改名，0600）。
// 调用者必须持有 s.mu 写锁，或能保证此时没有其他 goroutine 能拿到这个 Store
// （如 Open 初始化中，Store 还没进缓存）。
func (s *Store) saveLocked() error {
	s.doc.Version = storeVersion
	blob, err := json.MarshalIndent(s.doc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return err
	}
	tmp := filepath.Join(s.dir, storeFileName+".tmp")
	if err := os.WriteFile(tmp, blob, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(s.dir, storeFileName))
}

// save 加锁写盘。
func (s *Store) save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

// ---- Web 设置 ----

// LoadWebSettings 读当前 Web 设置并处理端口默认值与 -web 显式指定。
// 语义与原来一致：portExplicit 时以命令行为准并写回。
func (s *Store) LoadWebSettings(defaultPort int, portExplicit bool) (WebSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w := s.doc.Web
	needSave := false
	if w.Port == 0 {
		w.Port = defaultPort
		needSave = true
	}
	if portExplicit && w.Port != defaultPort {
		w.Port = defaultPort
		needSave = true
	}
	s.doc.Web = w
	if needSave {
		if err := s.saveLocked(); err != nil {
			return w, err
		}
	}
	return w, nil
}

// GetWebSettings 返回当前 Web 设置的拷贝。
func (s *Store) GetWebSettings() WebSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.doc.Web
}

// SaveWebSettings 把当前内存中的 Web 设置写盘（兼容老调用点）。
func (s *Store) SaveWebSettings() error { return s.save() }

// UpdateWebSettings 原子替换当前 Web 设置并落盘。
func (s *Store) UpdateWebSettings(next WebSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.doc.Web = next
	return s.saveLocked()
}

// SetSubToken 设置订阅口令并落盘。
func (s *Store) SetSubToken(tok string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.doc.Web.SubToken = tok
	return s.saveLocked()
}

// SetResidentialOnly 改"只用家宽"开关并落盘。
func (s *Store) SetResidentialOnly(v bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.doc.Web.ResidentialOnly = &v
	return s.saveLocked()
}

// ResidentialOnly 返回"只用家宽"是否开启（没配过默认开）。
func (s *Store) ResidentialOnly() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.doc.Web.residentialOnly()
}

// SetResidentialOnlyRaw 仅供测试：直接改内存值（可为 nil），不落盘；返回旧值。
func (s *Store) SetResidentialOnlyRaw(v *bool) *bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.doc.Web.ResidentialOnly
	s.doc.Web.ResidentialOnly = v
	return prev
}

// ---- 访问路径 ----

// LoadBasePath 读取或生成随机访问路径，形如 /aB3xY9pQ。
// 返回值 created 表示是不是这次新生成的。
func (s *Store) LoadBasePath() (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc.BasePath != "" {
		return s.doc.BasePath, false, nil
	}
	bp, err := randomBasePath(10)
	if err != nil {
		return "", false, err
	}
	s.doc.BasePath = normalizeBasePath(bp)
	if err := s.saveLocked(); err != nil {
		return "", false, fmt.Errorf("写访问路径失败: %w", err)
	}
	return s.doc.BasePath, true, nil
}

// CurrentBasePath 返回当前访问路径（形如 /xxx 或空）。
func (s *Store) CurrentBasePath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.doc.BasePath
}

// SetBasePath 校验并保存新的访问路径，立即生效。空串表示不加路径前缀。
func (s *Store) SetBasePath(raw string) (string, error) {
	bp := normalizeBasePath(raw)
	if bp != "" {
		if len(bp) > 100 {
			return "", fmt.Errorf("访问路径太长（最多100字符）")
		}
		// 用户手填的路径放宽到任意字母数字加 - _，不套用自动生成时刻意避开的
		// 易混字符集（那套是给随机生成用的）。
		for _, c := range strings.TrimPrefix(bp, "/") {
			ok := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
				(c >= '0' && c <= '9') || c == '-' || c == '_'
			if !ok {
				return "", fmt.Errorf("访问路径只能用字母、数字、- 和 _")
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.doc.BasePath = bp
	if err := s.saveLocked(); err != nil {
		return "", err
	}
	return bp, nil
}

// ---- 口令 ----

// Password 返回管理界面登录口令（可能为空，调用方负责生成）。
func (s *Store) Password() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.doc.Password
}

// SetPassword 设置登录口令并落盘。
func (s *Store) SetPassword(pw string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.doc.Password = strings.TrimSpace(pw)
	return s.saveLocked()
}

// ---- 会话 ----

// AddSession 新增一个登录会话并落盘，顺手清掉过期会话。
func (s *Store) AddSession(tok string, exp time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc.Sessions == nil {
		s.doc.Sessions = map[string]time.Time{}
	}
	now := time.Now()
	for k, e := range s.doc.Sessions {
		if now.After(e) {
			delete(s.doc.Sessions, k)
		}
	}
	s.doc.Sessions[tok] = exp
	return s.saveLocked()
}

// ValidSession 判断会话 token 是否有效（存在且未过期）。
func (s *Store) ValidSession(tok string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	exp, ok := s.doc.Sessions[tok]
	return ok && time.Now().Before(exp)
}

// DeleteSession 删除一个登录会话（登出），不存在也算成功。
func (s *Store) DeleteSession(tok string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc.Sessions == nil {
		return nil
	}
	delete(s.doc.Sessions, tok)
	return s.saveLocked()
}

// ClearSessions 清掉所有登录会话（改口令后踢掉所有人）。
func (s *Store) ClearSessions() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.doc.Sessions = map[string]time.Time{}
	return s.saveLocked()
}

// ---- 面板模式 ----

// PanelMode 返回界面选过的后端（3x-ui/native），空表示自动探测。
func (s *Store) PanelMode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.doc.PanelMode
}

// SetPanelMode 记录界面选的后端，空值表示回到自动探测。
func (s *Store) SetPanelMode(mode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.doc.PanelMode = strings.TrimSpace(mode)
	return s.saveLocked()
}

// ---- 3x-ui token ----

// XUIToken 返回上次落盘的 3x-ui 面板 API token，没有返回空串。
func (s *Store) XUIToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.doc.XUIToken
}

// SetXUIToken 落盘 3x-ui 面板 API token。
func (s *Store) SetXUIToken(tok string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.doc.XUIToken = strings.TrimSpace(tok)
	return s.saveLocked()
}

// ---- 隧道状态 ----

// SetTunnels 保存隧道状态的原始 JSON（core 包负责编解码）。
func (s *Store) SetTunnels(blob json.RawMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.doc.Tunnels = blob
	return s.saveLocked()
}

// Tunnels 返回隧道状态的原始 JSON，没存过返回 nil。
func (s *Store) Tunnels() json.RawMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.doc.Tunnels
}
