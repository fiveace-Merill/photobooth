export function isTypedConfirmationMatch(input: string, expected: string): boolean {
  return input.trim() === expected.trim() && expected.trim().length > 0;
}
