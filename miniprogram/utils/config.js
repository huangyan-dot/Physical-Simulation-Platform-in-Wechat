// utils/config.js - 全局可调参数（后端地址等）
//
// 【重要】真机 / 手机预览调试时不能用 localhost：
// localhost 指的是手机自己，不是你的电脑，手机连自己的 8080 端口什么都没有，
// 表现就是「网络请求失败」。模拟器能通是因为它和后端在同一台机器上。
// 必须填电脑的局域网 IP。查看方法：Windows 上执行 ipconfig，找「IPv4 地址」。
//
// 真机调试还需要三个条件同时满足：
//   1. 手机与电脑在同一个 WiFi（手机别开流量，公司/学校 WiFi 常做客户端隔离，连不通就开电脑热点）
//   2. 电脑防火墙放行 8080 入站（Windows 默认拦截，见下方命令）
//   3. 开发者工具「详情 → 本地设置」勾选「不校验合法域名、web-view、TLS 证书」
//      —— http 明文地址在真机上必须靠这个开关才允许请求
//
// 放行防火墙（管理员 PowerShell 执行一次）：
//   New-NetFirewallRule -DisplayName "physics-lab 8080" -Direction Inbound -Protocol TCP -LocalPort 8080 -Action Allow
//
// 上线前改为已备案的 https 域名，并在小程序后台配置 request 合法域名。
//
// 【换网/换地方必看】校园网、WiFi 重连后 DHCP 会重新分配 IP，
// 这里的 LAN_IP 一旦过期，所有请求都会超时（ERR_CONNECTION_TIMED_OUT →「网络请求失败」）。
// 只需 ipconfig 查到新的「IPv4 地址」，改下面这一行即可。
const LAN_IP = '10.17.251.22';

export const API_BASE_URL = `http://${LAN_IP}:8080`;

// 请求超时（毫秒）
export const REQUEST_TIMEOUT = 15000;
