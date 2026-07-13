import { useState } from 'react'
import { verifyToken } from './api'

export default function Login({ onSuccess }: { onSuccess: (token: string) => void }) {
  const [token, setToken] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    if (!token.trim()) {
      setError('Введите токен')
      return
    }
    setLoading(true)
    try {
      if (await verifyToken(token)) {
        onSuccess(token)
      } else {
        setError('Неверный токен')
      }
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <main className="wrap narrow">
      <h1>KeyChecker</h1>
      <form onSubmit={onSubmit}>
        <label>
          Токен доступа
          <input
            type="password"
            value={token}
            onChange={(e) => setToken(e.target.value)}
            placeholder="KEYCHECKER_TOKEN"
            autoFocus
          />
        </label>
        <button type="submit" disabled={loading}>
          {loading ? 'Проверяю…' : 'Войти'}
        </button>
      </form>
      {error && <p className="error">{error}</p>}
    </main>
  )
}
