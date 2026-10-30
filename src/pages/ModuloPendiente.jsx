import { Construction } from "lucide-react";
import { Badge, Card, EmptyState, PageHeader } from "../components/ui";

// Pantalla honesta para módulos aún no implementados: indica la etapa y los requisitos que cubrirá.
export const ModuloPendiente = ({ modulo: m }) => (
  <>
    <PageHeader
      title={m.etiqueta}
      description={m.requisitos ? `Requisitos: ${m.requisitos}` : undefined}
      actions={<Badge tone="brand">Etapa {m.etapa}</Badge>}
    />
    <Card>
      <EmptyState icon={<Construction className="size-8" />} title="Este módulo se implementa en una etapa posterior">
        <p>{m.resumen}</p>
        <p className="mt-2">
          Su perfil ya tiene permiso para usarlo; la opción se habilitará al completar la Etapa {m.etapa}.
        </p>
      </EmptyState>
    </Card>
  </>
);

export const NoEncontrada = () => (
  <Card>
    <EmptyState title="Página no encontrada">La dirección no existe o no tiene permiso para verla.</EmptyState>
  </Card>
);
