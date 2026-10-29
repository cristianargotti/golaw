export function totalValue(rows) {
  let sum = 0;
  for (const row of rows) {
    sum += row.value;
  }
  return sum;
}

export function totalWeight(rows) {
  let sum = 0;
  for (const row of rows) {
    sum += row.weight;
  }
  return sum;
}
