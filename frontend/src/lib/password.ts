/** The Mango3S server's password rules (the backend checks the same ones
 * before sending; the server has the final say and may ask for more length). */
export const MIN_PASSWORD_LENGTH = 10

export function passwordProblems(password: string, email = ''): string[] {
  const problems: string[] = []
  if ([...password].length < MIN_PASSWORD_LENGTH) problems.push(`Use at least ${MIN_PASSWORD_LENGTH} characters (12 or more is best).`)
  const classes = [/[a-z]/, /[A-Z]/, /[0-9]/, /[^A-Za-z0-9]/].filter((re) => re.test(password)).length
  if (classes < 3) problems.push('Mix at least three of: lowercase, uppercase, numbers, symbols.')
  const local = email.trim().toLowerCase().split('@')[0] ?? ''
  if (local.length >= 3 && password.toLowerCase().includes(local)) problems.push("Don't include your email name.")
  if (/(.)\1{3,}/.test(password)) problems.push('Avoid repeating the same character.')
  if (/password|qwerty|123456|mango3s/i.test(password)) problems.push('Avoid common words and sequences.')
  return problems
}
