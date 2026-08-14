// utils/storage.js - 本地存储封装

const KEYS = {
  TOKEN: 'physics_token',
  USER_INFO: 'physics_user_info',
  SETTINGS: 'physics_settings'
};

export function setToken(token) {
  wx.setStorageSync(KEYS.TOKEN, token);
}

export function getToken() {
  return wx.getStorageSync(KEYS.TOKEN);
}

export function removeToken() {
  wx.removeStorageSync(KEYS.TOKEN);
}

export function setUserInfo(userInfo) {
  wx.setStorageSync(KEYS.USER_INFO, userInfo);
}

export function getUserInfo() {
  return wx.getStorageSync(KEYS.USER_INFO);
}

export function removeUserInfo() {
  wx.removeStorageSync(KEYS.USER_INFO);
}

export function clearAll() {
  wx.removeStorageSync(KEYS.TOKEN);
  wx.removeStorageSync(KEYS.USER_INFO);
  wx.removeStorageSync(KEYS.SETTINGS);
}
