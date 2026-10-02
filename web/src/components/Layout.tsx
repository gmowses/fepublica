import { Link, NavLink } from "react-router-dom";
import { Menu, Moon, Sun, X, Github } from "lucide-react";
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
    document.documentElement.classList.contains("light") ? "light" : "dark",
  );
  const [isNavOpen, setIsNavOpen] = useState(false);

  useEffect(() => {
    if (theme === "light") document.documentElement.classList.add("light");
    else document.documentElement.classList.remove("light");
    localStorage.setItem("fepublica-theme", theme);
  }, [theme]);

  return (
    <header className="sticky top-0 z-30 border-b border-ink/15 bg-bg/90 backdrop-blur">
      <div className="container-app flex h-16 items-center justify-between gap-3">
        <Link to="/" className="flex min-w-0 items-center gap-2">
          <Logo />
          <span className="truncate font-display font-bold tracking-tight">
            Fé Pública
          </span>
          <span className="hidden sm:inline text-xs font-mono text-ink-dim">
            1.6
          </span>
        </Link>
        <nav
          className="hidden md:flex items-center gap-1 text-sm"
          aria-label="Navegação principal"
        >
          <NavigationItems />
        </nav>
        <div className="flex items-center gap-1">
          <a
            href="https://github.com/gmowses/fepublica"
            target="_blank"
            rel="noopener"
            className="rounded-md p-2 hover:bg-bg-soft"
            aria-label="Repositório no GitHub"
          >
            <Github className="size-4" />
          </a>
          <button
            type="button"
            onClick={() => setTheme(theme === "light" ? "dark" : "light")}
            className="rounded-md p-2 hover:bg-bg-soft"
            aria-label="Alternar tema"
          >
            {theme === "light" ? (
              <Moon className="size-4" />
            ) : (
              <Sun className="size-4" />
            )}
          </button>
          <button
            type="button"
            onClick={() => setIsNavOpen((open) => !open)}
            className="rounded-md p-2 hover:bg-bg-soft md:hidden"
            aria-label={
              isNavOpen ? "Fechar menu de navegação" : "Abrir menu de navegação"
            }
            aria-controls="mobile-navigation"
            aria-expanded={isNavOpen}
          >
            {isNavOpen ? <X className="size-4" /> : <Menu className="size-4" />}
          </button>
        </div>
      </div>
      {isNavOpen && (
        <nav
          id="mobile-navigation"
          className="container-app grid grid-cols-2 gap-1 border-t border-ink/15 py-3 text-sm md:hidden"
          aria-label="Navegação principal"
        >
          <NavigationItems onNavigate={() => setIsNavOpen(false)} />
        </nav>
      )}
    </header>
  );
}

function NavigationItems({ onNavigate }: { onNavigate?: () => void }) {
  return (
    <>
      <NavItem to="/" onNavigate={onNavigate}>
        Início
      </NavItem>
      <NavItem to="/gastos" onNavigate={onNavigate}>
        Gastos
      </NavItem>
      <NavItem to="/forenses" onNavigate={onNavigate}>
        Forenses
      </NavItem>
      <NavItem to="/observatorio" onNavigate={onNavigate}>
        Observatório
      </NavItem>
      <NavItem to="/entes" onNavigate={onNavigate}>
        Entes
      </NavItem>
      <NavItem to="/recent" onNavigate={onNavigate}>
        Recentes
      </NavItem>
      <NavItem to="/about" onNavigate={onNavigate}>
        Sobre
      </NavItem>
    </>
  );
}

function NavItem({
  to,
  children,
  onNavigate,
}: {
  to: string;
  children: React.ReactNode;
  onNavigate?: () => void;
}) {
  return (
    <NavLink
      to={to}
      end
      onClick={onNavigate}
      className={({ isActive }) =>
        "rounded-md px-3 py-2 transition-colors " +
        (isActive
          ? "bg-bg-soft text-accent"
          : "text-ink hover:bg-bg-soft hover:text-accent")
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
    <footer className="mt-12 border-t border-ink/15 bg-bg-soft">
      <div className="container-app flex flex-col justify-between gap-3 py-7 text-xs leading-5 text-ink-dim sm:flex-row">
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
