import React from "react";

// Shared only by semantically equivalent, read-only resource collections.
// Callers retain row rendering and evidence ownership; this primitive owns
// native table semantics and the focusable horizontal-overflow region.
export function ReadOnlyCollectionTable({label, caption, columns, className="", children}) {
  return <div className="table-scroll collection-table-region" tabIndex="0" role="region" aria-label={label}>
    <table className={`collection-table${className ? ` ${className}` : ""}`}>
      <caption className="visually-hidden">{caption}</caption>
      <thead><tr>{columns.map((column, index) => <th scope="col" key={column.key ?? index}>{column.label}</th>)}</tr></thead>
      <tbody>{children}</tbody>
    </table>
  </div>;
}

export function OffsetPagination({label, offset, limit, returned, total, previousLabel, nextLabel, onPrevious, onNext}) {
  const end = offset + returned;
  return <nav className="pagination collection-pagination" aria-label={label}>
    <button disabled={offset === 0} onClick={onPrevious}>{previousLabel}</button>
    <span aria-live="polite">{returned ? `${offset + 1}–${end}` : "0"} / {total}</span>
    <button disabled={end >= total} onClick={onNext}>{nextLabel}</button>
  </nav>;
}
