import { useState } from 'react'
import Login from './Login'
import Checker from './Checker'
import './styles.css'

const TOKEN_KEY = 'keychecker_token'

export default function App() {
  const [token, setToken] = useState<string>(() => sessionStorage.getItem(TOKEN_KEY) ?? '')

  function login(t: string) {
    sessionStorage.setItem(TOKEN_KEY, t)
    setToken(t)
  }

  function logout() {
    sessionStorage.removeItem(TOKEN_KEY)
    setToken('')
  }

  return token ? <Checker token={token} onLogout={logout} /> : <Login onSuccess={login} />
}
