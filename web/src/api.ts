export type Balance = { amount: number; currency: string; limit?: number }
export type Result = {
  key: string
  provider: string
  valid: boolean
  balance?: Balance
  detail: string
}

export async function checkKeys(token: string, keys: string[]): Promise<Result[]> {
  const resp = await fetch('/api/check', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify({ keys }),
  })
  if (resp.status === 401) throw new Error('Неверный токен доступа')
  if (!resp.ok) throw new Error(`Ошибка сервера: ${resp.status}`)
  const data = await resp.json()
  return data.results as Result[]
}
