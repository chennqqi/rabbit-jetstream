const catalogs = Object.freeze({en: key => `Remove label ${JSON.stringify(key)} from this draft?`, zh: key => `从草稿移除标签 ${JSON.stringify(key)}？`});
export function removeLabelPrompt(language, key) { return (catalogs[language] ?? catalogs.en)(key); }
