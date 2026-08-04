// utils/config.js - 全局可调参数（后端地址等）
//
// 【重要】真机 / 手机预览调试时不能用 localhost：
// localhost 指的是手机自己，不是你的电脑。请改成电脑的局域网 IP，例如：
//   export const API_BASE_URL = 'http://192.168.1.100:8080';
// 查看电脑 IP：Windows 上执行 ipconfig，找「IPv4 地址」。
// 手机需与电脑处于同一 WiFi，且电脑防火墙允许 8080 端口。
//
// 上线前改为已备案的 https 域名，并在小程序后台配置 request 合法域名。
export const API_BASE_URL = 'http://localhost:8080';

// 请求超时（毫秒）
export const REQUEST_TIMEOUT = 15000;
