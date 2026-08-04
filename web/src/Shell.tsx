import { Component, useEffect, useState, type ReactNode } from "react";
import { api, clearToken, getToken, setToken } from "./lib";
import { Button, Field, Input, Panel } from "./ui";

/** A thrown render error used to blank the whole console. One bad payload in
 *  one panel should not take the rest of the company's controls with it. */
export class ErrorBoundary extends Component<
  { children: ReactNode },
  { error: Error | null }
> {
  state: { error: Error | null } = { error: null };

  static getDerivedStateFromError(error: Error) {
    return { error };
  }

  render() {
    if (!this.state.error) return this.props.children;
    return (
      <div className="p-6">
        <Panel title="This panel crashed">
          <pre className="font-mono text-[12px] whitespace-pre-wrap text-alert">
            {this.state.error.message}
          </pre>
          <div className="mt-3">
            <Button onClick={() => this.setState({ error: null })}>try again</Button>
          </div>
        </Panel>
      </div>
    );
  }
}

/** The API can hire agents and read the whole shared memory, so it is not open.
 *  iofi prints a token at startup when IOFI_AUTH_TOKEN is unset. */
export function TokenGate({ children }: { children: ReactNode }) {
  const [authed, setAuthed] = useState(!!getToken());
  const [value, setValue] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    const onDenied = () => setAuthed(false);
    addEventListener("iofi:unauthorized", onDenied);
    return () => removeEventListener("iofi:unauthorized", onDenied);
  }, []);

  if (authed) {
    return (
      <>
        {children}
        <button
          onClick={() => {
            clearToken();
            setAuthed(false);
          }}
          className="fixed right-3 bottom-3 rounded-md border border-ink-800 bg-ink-900 px-2 py-1 text-[11px] text-ink-500 hover:text-ink-100"
        >
          sign out
        </button>
      </>
    );
  }

  const submit = async () => {
    setBusy(true);
    setError(null);
    setToken(value.trim());
    try {
      await api("/dashboard");
      setAuthed(true);
    } catch {
      clearToken();
      setError("That token was rejected.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex min-h-full items-center justify-center p-6">
      <div className="w-full max-w-sm space-y-4">
        <div>
          <h1 className="text-lg font-semibold tracking-tight">iofi</h1>
          <p className="mt-1 text-[12px] text-ink-500">
            Enter the access token. It is printed in the server log at startup
            when <code className="font-mono">IOFI_AUTH_TOKEN</code> is not set.
          </p>
        </div>
        <Field label="Access token">
          <Input
            type="password"
            value={value}
            autoFocus
            onChange={(e) => setValue(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && submit()}
          />
        </Field>
        {error && <p className="text-[12px] text-alert">{error}</p>}
        <Button variant="primary" onClick={submit} disabled={busy || !value.trim()} className="w-full">
          {busy ? "checking…" : "enter"}
        </Button>
      </div>
    </div>
  );
}
