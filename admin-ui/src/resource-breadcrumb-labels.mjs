const catalogs = Object.freeze({en: "Breadcrumb", zh: "面包屑"});
export function resourceBreadcrumbLabel(language) { return catalogs[language] ?? catalogs.en; }
