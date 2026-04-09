import { Link, NavLink } from "react-router-dom";
import { Moon, Sun, Github } from "lucide-react";
import { useEffect, useState } from "react";

export function Layout({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen flex flex-col">
      <Header />
      <main className="flex-1">{children}</main>
      <Footer />
    </div>
  );
}

function Header() {
  const [theme, setTheme] = useState<"dark" | "light">(() =>
    document.documentElement.classList.contains("light") ? "light" : "dark"
  );

  useEffect(() => {
    if (theme === "light") document.documentElement.classList.add("light");
    else document.documentElement.classList.remove("light");
    localStorage.setItem("fepublica-theme", theme);
  }, [theme]);

  return (
    <header className="border-b border-ink/10 sticky top-0 z-30 backdrop-blur bg-bg/80">
      <div className="container-app flex items-center justify-between h-14 gap-4">
        <Link to="/" className="flex items-center gap-2">
          <Logo />
          <span className="font-semibold tracking-tight">Fé Pública</span>
          <span className="hidden sm:inline text-xs font-mono text-ink-dim">
            alpha
          </span>
        </Link>
        <nav className="flex items-center gap-1 text-sm">
          <NavItem to="/">Início</NavItem>
          <NavItem to="/recent">Recentes</NavItem>
          <NavItem to="/about">Sobre</NavItem>
          <a
            href="https://github.com/gmowses/fepublica"
            target="_blank"
            rel="noopener"
            className="ml-2 p-2 rounded-md hover:bg-bg-card"
            aria-label="Repositório no GitHub"
          >
            <Github className="size-4" />
          </a>
          <button
            type="button"
            onClick={() => setTheme(theme === "light" ? "dark" : "light")}
            className="p-2 rounded-md hover:bg-bg-card"
            aria-label="Alternar tema"
          >
            {theme === "light" ? (
              <Moon className="size-4" />
            ) : (
              <Sun className="size-4" />
            )}
          </button>
        </nav>
      </div>
    </header>
  );
}

function NavItem({
  to,
  children,
}: {
  to: string;
  children: React.ReactNode;
}) {
  return (
    <NavLink
      to={to}
      end
      className={({ isActive }) =>
        "px-3 py-1.5 rounded-md transition " +
        (isActive
          ? "text-accent bg-bg-card"
          : "text-ink hover:bg-bg-card hover:text-accent")
      }
    >
      {children}
    </NavLink>
  );
}

function Logo() {
  return (
    <svg
      viewBox="0 0 24 24"
      className="size-6 text-accent"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <circle cx="12" cy="5" r="2.5" />
      <path d="M12 7.5v3" />
      <path d="M6 10.5h12" />
      <path d="M12 10.5v8" />
      <path d="M7 18.5c0 2 2.2 2.5 5 2.5s5-.5 5-2.5" />
    </svg>
  );
}

function Footer() {
  return (
    <footer className="border-t border-ink/10 mt-12">
      <div className="container-app py-6 text-xs text-ink-dim flex flex-col sm:flex-row gap-3 justify-between">
        <div>
          Fé Pública · software livre sob{" "}
          <a
            className="underline hover:text-accent"
            href="https://github.com/gmowses/fepublica/blob/main/LICENSE"
            target="_blank"
            rel="noopener"
          >
            AGPL-3.0
          </a>
          . Código em{" "}
          <a
            className="underline hover:text-accent"
            href="https://github.com/gmowses/fepublica"
            target="_blank"
            rel="noopener"
          >
            github.com/gmowses/fepublica
          </a>
          .
        </div>
        <div>
          Não afiliado à CGU, PNCP ou qualquer órgão público. Dados coletados
          via APIs oficiais sob a Lei de Acesso à Informação.
        </div>
      </div>
    </footer>
  );
}
