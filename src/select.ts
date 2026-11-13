export type SearchableOption = {
  value: string | number;
  label: string;
  disabled?: boolean;
  keywords?: string;
};

export function matchesOption(option: SearchableOption, query: string): boolean {
  const tokens = query.trim().toLowerCase().split(/\s+/).filter(Boolean);
  if (!tokens.length) return true;
  const haystack = `${option.label} ${option.keywords || ''} ${option.value}`.toLowerCase();
  return tokens.every(token => haystack.includes(token));
}
