import { Chat } from "./components/Chat";
import { Login } from "./components/Login";
import { useAuth } from "./hooks/useAuth";

export function App() {
  const { status, user, error, signIn, signOut } = useAuth();

  if (status === "loading") {
    return <div className="auth-loading" />;
  }

  if (status === "anonymous") {
    return <Login onSignIn={signIn} error={error} />;
  }

  return <Chat user={user} onSignOut={signOut} />;
}
