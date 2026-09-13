const catalogs = Object.freeze({en: Object.freeze({unknown: "Unknown", copy: "Copy", copied: "Copied", copiedStatus: "Copied to clipboard"}), zh: Object.freeze({unknown: "未知", copy: "复制", copied: "已复制", copiedStatus: "已复制到剪贴板"})});
export function commonDisplayLabels(language) { return catalogs[language] ?? catalogs.en; }
