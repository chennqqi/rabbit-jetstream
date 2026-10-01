import React from "react";
import {resourceBreadcrumbLabel} from "./resource-breadcrumb-labels.mjs";

// Shared resource breadcrumb. The trail is the only "back to list"
// affordance that works from a deep link or a shared URL. The last segment
// is the current location and is not a link.
export function ResourceBreadcrumb({trail, router, language}) {
  return <nav className="resource-breadcrumb" aria-label={resourceBreadcrumbLabel(language)}>
    <ol>{trail.map((segment, index) => {
      const current = index === trail.length - 1;
      const visit = event => {
        if (event.button === 0 && !event.ctrlKey && !event.metaKey && !event.shiftKey && !event.altKey) {
          event.preventDefault();
          router.navigate(segment.href);
        }
      };
      return <li key={index}>{current ? <span aria-current="location">{segment.label}</span> : <a href={segment.href} onClick={visit}>{segment.label}</a>}</li>;
    })}</ol>
  </nav>;
}
