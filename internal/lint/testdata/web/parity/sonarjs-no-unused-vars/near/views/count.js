export function count(rows) {
  const present = rows.filter(Boolean);
  return present.length;
}
