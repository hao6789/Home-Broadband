package core

import (
	"home-broadband/internal/config"
	"home-broadband/internal/job"
	"home-broadband/internal/vpngate"
)

// 过渡别名：结构重构期间让各文件函数体保持零改动，后续可逐步去掉。
type (
	Node     = vpngate.Node
	Job      = job.Job
	JobStore = job.JobStore
	JobView  = job.JobView
	JobStep  = job.JobStep
)

// 过渡函数别名：同上。
var (
	FetchNodes       = vpngate.FetchNodes
	NodeLabel        = vpngate.NodeLabel
	CountryLabel     = vpngate.CountryLabel
	UniqueRemark     = vpngate.UniqueRemark
	IsGeneratedLabel = vpngate.IsGeneratedLabel

	ResidentialOnly = config.ResidentialOnly
)
