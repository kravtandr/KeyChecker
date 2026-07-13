import { useState } from 'react'
import { checkKeys, type Result } from './api'
import './styles.css'

export default function App() {
  const [token, setToken] = useState('')
  const [raw, setRaw] = useState('')
  const [results, setResults] = useState<Result[]>([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  async function onCheck() {
    setError('')
    setResults([])
    const keys = raw.split('\n').map((s) => s.trim()).filter(Boolean)
    if (keys.length === 0) {
      setError('Введите хотя бы один ключ')
      return
    }
    setLoading(true)
    try {
      setResults(await checkKeys(token, keys))
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <main className="wrap">
      <h1>KeyChecker</h1>
      <label>
        Токен доступа
        <input
          type="password"
          value={token}
          onChange={(e) => setToken(e.target.value)}
          placeholder="KEYCHECKER_TOKEN"
        />
      </label>
      <label>
        Ключи (по одному на строку)
        <textarea
          rows={8}
          value={raw}
          onChange={(e) => setRaw(e.target.value)}
          placeholder={'sk-...\nsk-ant-...\nsk-or-v1-...'}
        />
      </label>
      <button onClick={onCheck} disabled={loading}>
        {loading ? 'Проверяю…' : 'Проверить'}
      </button>
      {error && <p className="error">{error}</p>}
      {results.length > 0 && (
        <table>
          <thead>
            <tr>
              <th>Ключ</th>
              <th>Провайдер</th>
              <th>Статус</th>
              <th>Баланс</th>
              <th>Детали</th>
            </tr>
          </thead>
          <tbody>
            {results.map((r, i) => (
              <tr key={i}>
                <td className="mono">{r.key}</td>
                <td>{r.provider}</td>
                <td className={r.valid ? 'ok' : 'bad'}>{r.valid ? '✓ валиден' : '✗ невалиден'}</td>
                <td>
                  {r.balance
                    ? `${r.balance.amount} ${r.balance.currency}` +
                      (r.balance.limit != null ? ` / ${r.balance.limit}` : '')
                    : '—'}
                </td>
                <td>{r.detail}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </main>
  )
}
