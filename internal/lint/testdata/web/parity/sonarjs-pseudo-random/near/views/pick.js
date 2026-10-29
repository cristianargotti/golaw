export const pick = (rows) => rows[crypto.getRandomValues(new Uint32Array(1))[0] % rows.length];
