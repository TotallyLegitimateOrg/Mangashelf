import { useState, useEffect, type FormEvent } from "react";
import { useNavigate } from "react-router";
import { useAuth } from "@/lib/auth";
import * as api from "@/lib/api";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { useToast } from "@/components/ui/Toast";
import "./LoginPage.css";

export default function LoginPage() {
  const { loginWithToken, isAuthenticated } = useAuth();
  const navigate = useNavigate();
  const { toast } = useToast();
  const [needsSetup, setNeedsSetup] = useState<boolean | null>(null);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (isAuthenticated) {
      navigate("/", { replace: true });
      return;
    }
    api.checkNeedsSetup().then(setNeedsSetup).catch(() => setNeedsSetup(false));
  }, [isAuthenticated, navigate]);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError("");

    if (needsSetup && password !== confirmPassword) {
      setError("Passwords don't match");
      return;
    }
    if (needsSetup && password.length < 6) {
      setError("Password must be at least 6 characters");
      return;
    }

    setLoading(true);
    try {
      const result = needsSetup
        ? await api.setup(username, password)
        : await api.login(username, password);
      loginWithToken(result.token, result.user);
      toast(needsSetup ? "Account created! Welcome." : "Welcome back!", "success");
      navigate("/", { replace: true });
    } catch (err) {
      setError(api.errorMessage(err, "Failed to sign in"));
    } finally {
      setLoading(false);
    }
  };

  if (needsSetup === null) return null;

  return (
    <div className="login-page">
      {/* Background decorations */}
      <div className="login-page__bg">
        <div className="login-page__orb login-page__orb--1" />
        <div className="login-page__orb login-page__orb--2" />
        <div className="login-page__orb login-page__orb--3" />
      </div>

      <div className="login-card">
        <div className="login-card__header">
          <span className="login-card__logo">📖</span>
          <h1 className="login-card__title font-display">Mangashelf</h1>
          <p className="login-card__subtitle">
            {needsSetup
              ? "Create your account to get started"
              : "Sign in to your library"}
          </p>
        </div>

        <form className="login-card__form" onSubmit={handleSubmit}>
          <Input
            label="Username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            autoFocus
            autoComplete="username"
            required
          />
          <Input
            label="Password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete={needsSetup ? "new-password" : "current-password"}
            required
          />
          {needsSetup && (
            <Input
              label="Confirm Password"
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              autoComplete="new-password"
              required
            />
          )}

          {error && <p className="login-card__error">{error}</p>}

          <Button type="submit" loading={loading} size="lg">
            {needsSetup ? "Create Account" : "Sign In"}
          </Button>
        </form>

      </div>
    </div>
  );
}
