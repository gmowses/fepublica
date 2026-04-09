import { Routes, Route } from "react-router-dom";
import { Layout } from "./components/Layout";
import { Landing } from "./pages/Landing";
import { SnapshotDetail } from "./pages/SnapshotDetail";
import { EventDetail } from "./pages/EventDetail";
import { DiffViewer } from "./pages/DiffViewer";
import { SourceDetail } from "./pages/SourceDetail";
import { Recent } from "./pages/Recent";
import { About } from "./pages/About";
import { NotFound } from "./pages/NotFound";
import { Observatorio } from "./pages/Observatorio";
import { Entes } from "./pages/Entes";
import { EnteDetail } from "./pages/EnteDetail";
import { Gastos } from "./pages/Gastos";
import { Fornecedor } from "./pages/Fornecedor";

export default function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Landing />} />
        <Route path="/snapshots/:id" element={<SnapshotDetail />} />
        <Route
          path="/snapshots/:id/events/:externalId"
          element={<EventDetail />}
        />
        <Route path="/diff/:a/:b" element={<DiffViewer />} />
        <Route path="/sources/:id" element={<SourceDetail />} />
        <Route path="/recent" element={<Recent />} />
        <Route path="/observatorio" element={<Observatorio />} />
        <Route path="/entes" element={<Entes />} />
        <Route path="/entes/:id" element={<EnteDetail />} />
        <Route path="/gastos" element={<Gastos />} />
        <Route path="/gastos/fornecedores/:ni" element={<Fornecedor />} />
        <Route path="/about" element={<About />} />
        <Route path="*" element={<NotFound />} />
      </Routes>
    </Layout>
  );
}
