import React from "react";

// Shared internal navigation link: intercepts plain left clicks and routes
// in-app, preserving modified clicks and auxiliary buttons for the browser.
export function RouteLink({href, router, children, ...rest}) {
  return <a href={href} {...rest} onClick={event => {
    if (event.button === 0 && !event.ctrlKey && !event.metaKey && !event.shiftKey && !event.altKey) {
      event.preventDefault();
      router.navigate(href);
    }
  }}>{children}</a>;
}
