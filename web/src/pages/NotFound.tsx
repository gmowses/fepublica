import { Link } from "react-router-dom";

export function NotFound() {
  return (
    <div className="container-app py-24 text-center">
      <h1 className="text-4xl font-bold mb-2">404</h1>
      <p className="text-ink-dim mb-6">Página não encontrada.</p>
      <Link to="/" className="btn">
        ← voltar ao início
      </Link>
    </div>
  );
}
