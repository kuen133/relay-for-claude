package main

import "strings"

var chineseLabels = map[string]string{
	"Auto-switch when nearly full":             "达到阈值时自动切换账号",
	"Auto-switch at":                           "自动切换阈值",
	"Use 5-hour limit only":                    "仅按五小时额度触发",
	"Sync history from all accounts":           "合并所有账号的项目与会话",
	"Auto-switch to use up expiring weekly":    "优先使用即将重置的周额度",
	"Add account…":                             "添加账号…",
	"Import chats from all accounts":           "立即合并所有账号的会话",
	"Start at login":                           "开机启动",
	"Open data folder":                         "打开数据文件夹",
	"Quit":                                     "退出",
	"Settings":                                 "设置",
	"Add account":                              "添加账号",
	"Import":                                   "合并会话",
	"signed out, add it again":                 "登录已失效，请重新添加",
	"Open Claude and sign in":                  "请打开 Claude 并登录",
	"Sign in with the other account in Claude": "请在 Claude 登录另一个账号",
	"Importing chats…":                         "正在合并会话…",
}

func translate(lang, s string) string {
	if lang == "zh-CN" {
		if v, ok := chineseLabels[s]; ok {
			return v
		}
	}
	return s
}

func localizedUsage(lang, s string) string {
	if lang != "zh-CN" {
		return s
	}
	return strings.NewReplacer("usage unknown", "用量未知", "5h ", "五小时 ", "week ", "本周 ", " until ", "，重置于 ", ", resets ", "，重置于 ").Replace(s)
}
