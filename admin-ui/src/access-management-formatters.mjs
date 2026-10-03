const catalogs = Object.freeze({en: Object.freeze({deleteAccount: name => `Delete ${name}?`, roleIn: tenant => `Role in ${tenant}`}), zh: Object.freeze({deleteAccount: name => `删除 ${name}？`, roleIn: tenant => `${tenant} 中的角色`})});
export function accessManagementFormatters(language) { return catalogs[language] ?? catalogs.en; }
