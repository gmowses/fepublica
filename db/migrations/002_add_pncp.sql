-- Fé Pública migration: add PNCP contracts source

INSERT INTO sources (id, name, base_url, description) VALUES
    ('pncp-contratos',
     'PNCP — Contratos Públicos',
     'https://pncp.gov.br/api/consulta/v1/contratos',
     'Contratos públicos federais, estaduais e municipais agregados pelo Portal Nacional de Contratações Públicas desde 2023. Base legal: Lei 14.133/2021 (Nova Lei de Licitações).')
ON CONFLICT (id) DO NOTHING;
