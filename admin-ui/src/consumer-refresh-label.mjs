const formatters = Object.freeze({
  en: seconds => `Automatic collection refresh: ${seconds} seconds after each read; failure backoff up to 60 seconds.`,
  zh: seconds => `集合自动刷新：每次读取完成后 ${seconds} 秒；失败退避最长 60 秒。`,
});
export function consumerRefreshLabel(language, seconds) { return (formatters[language] ?? formatters.en)(seconds); }
