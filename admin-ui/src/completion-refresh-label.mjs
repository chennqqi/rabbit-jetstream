const formatters = Object.freeze({en: seconds => `Refresh ${seconds}s after completion; failures back off up to 60s.`, zh: seconds => `完成后 ${seconds} 秒刷新，失败退避最长 60 秒。`});
export function completionRefreshLabel(language, seconds) { return (formatters[language] ?? formatters.en)(seconds); }
