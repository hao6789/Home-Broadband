package web

import (
	"home-broadband/internal/config"
	"home-broadband/internal/core"
	"home-broadband/internal/job"
	"home-broadband/internal/vpngate"
)

// 过渡别名：结构重构期间让各文件函数体保持零改动，后续可逐步去掉。
type (
	Manager          = core.Manager
	Tunnel           = core.Tunnel
	SocksCred        = core.SocksCred
	Panel            = core.Panel
	XUI              = core.XUI
	Inbound          = core.Inbound
	InboundDetail    = core.InboundDetail
	InboundPatch     = core.InboundPatch
	CreatedInbound   = core.CreatedInbound
	NewInboundSpec   = core.NewInboundSpec
	ProvisionRequest = core.ProvisionRequest
	Exit             = core.Exit
	ExitsView        = core.ExitsView
	RegionStat       = core.RegionStat
	Node             = vpngate.Node
	Job              = job.Job
	JobView          = job.JobView
	WebSettings      = config.WebSettings
)

// 过渡函数别名：同上。
var (
	OpenPanel           = core.OpenPanel
	SwitchPanelMode     = core.SwitchPanelMode
	CurrentPanelMode    = core.CurrentPanelMode
	AvailablePanelModes = core.AvailablePanelModes
	CachedInbounds      = core.CachedInbounds
	InvalidateInbounds  = core.InvalidateInbounds
	FirstLine           = core.FirstLine
	HostPublicIP        = core.HostPublicIP

	GetWebSettings      = config.GetWebSettings
	SaveWebSettings     = config.SaveWebSettings
	SetResidentialOnly  = config.SetResidentialOnly
	ResidentialOnly     = config.ResidentialOnly
	CurrentBasePath     = config.CurrentBasePath
	SetBasePath         = config.SetBasePath
	ValidatePort        = config.ValidatePort
	NormalizeListenAddr = config.NormalizeListenAddr

	sanitizeTag = core.SanitizeTag
)
