import { useState } from 'react'
import { checkKeys, type Result } from './api'

export default function Checker({ token, onLogout }: { token: string; onLogout: () => void }) {
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
      const msg = (e as Error).message
      setError(msg)
      // токен протух/отозван — возвращаем на экран входа
      if (msg.includes('токен')) onLogout()
    } finally {
      setLoading(false)
    }
  }

  return (
    <main className="wrap">
      <div className="topbar">
        <h1>KeyChecker</h1>
        <button className="link" onClick={onLogout}>
          Выйти
        </button>
      </div>
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
