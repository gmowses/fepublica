---
title: "Building a Bitcoin-anchored archive of Brazilian public data"
published: false
description: "How I built Fé Pública — a verifiable archive of Brazilian government data, anchored in Bitcoin via OpenTimestamps. Open source, runs on a single VPS."
tags: go, opensource, blockchain, govtech
canonical_url: https://github.com/gmowses/fepublica
cover_image:
---

# Building a Bitcoin-anchored archive of Brazilian public data

Brazilian public data has a quiet integrity problem. Companies disappear from sanctions lists with no audit log. Contracts get reclassified silently. PDFs become 404s. By the time anyone notices, there's no way to prove what the official record looked like yesterday.

I shipped **[Fé Pública](https://fepublica.gmowses.cloud)** to fix that — an archive of Brazilian public data that's verifiable cryptographically and anchored in Bitcoin. Code is on [GitHub](https://github.com/gmowses/fepublica) under AGPL-3.0.

## What it does

The system runs three loops continuously:

**1. Collection.** Workers scrape data from official APIs — CEIS and CNEP (the federal sanctions lists from CGU), PNCP contratos (the federal public procurement portal), and CPGF (federal corporate card transactions). Right now there are ~27k sanction records, 1.6k tracked contracts (R$ 47M+) and growing.

**2. Sealing.** Every collection becomes a "snapshot". The snapshot's records are canonicalized as JSON, hashed with SHA-256, and assembled into a Merkle tree. The Merkle root is then submitted to three OpenTimestamps calendars (alice, bob, finney), which periodically commit aggregate roots to Bitcoin. Result: each snapshot has a tamper-evident proof you can verify offline, against the public Bitcoin blockchain, without trusting the Fé Pública server to stay online.

**3. Cross-checking.** When a CNPJ from a procurement contract matches a CNPJ on the sanctions lists, the contract gets a red flag in the UI. Citizens, journalists, and prosecutors get a single place to see "this company was barred from contracting with the federal government, but state X just signed a R$ 8M contract with them anyway".

## The architecture

```
┌─────────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
│ collector   │  │ anchor   │  │  driftd  │  │ notifier │
│  (workers)  │  │ (OTS)    │  │ (changes)│  │ (alerts) │
└──────┬──────┘  └────┬─────┘  └────┬─────┘  └─────┬────┘
       │              │             │              │
       └──────────────┴─────────────┴──────────────┘
                            │
                       ┌────▼────┐
                       │postgres │  ← append-only triggers,
                       │   16    │     events table is
                       └────┬────┘     immutable by design
                            │
                       ┌────▼────┐
                       │   api   │  ← Go HTTP, embeds the
                       │ (Go)    │     React SPA via embed.FS
                       └─────────┘
```

Append-only enforcement is done at the database level — there are triggers on `events`, `snapshots`, and `anchors` that block all UPDATE and DELETE operations. The only writes the API service can do are inserts. If someone gets a shell on the box and wants to rewrite history, they'd have to reach into the database with `psql` and disable the trigger first — and that's auditable.

OpenTimestamps was the right call for the anchoring layer because the math is the same as Bitcoin's own consensus security, the calendars are run by independent operators, and the proofs are tiny — a few hundred bytes per snapshot. No layer-2, no oracle, no smart contract, no token. Just `H(data) → calendar → bitcoin`.

## The fun bits

Some things I learned in the implementation:

**PNCP is unreliable on purpose.** The federal procurement API regularly returns HTTP 500 with "Erro na comunicação com o banco de dados" — sometimes for hours. The fetcher retries with exponential backoff up to 5 attempts and 60s of total backoff between pages. On a typical day a 1500-record collection takes 30-40 minutes because every few pages hit a transient backend failure.

**The dedup field matters.** PNCP's response has two `numeroControle*` fields: `numeroControlePNCP` is the stable ID of a specific contract, and `numeroControlePncpCompra` is the parent purchase order — which is shared by all contracts derived from it. Reading the wrong one collapsed 1550 records into 81 unique IDs after the database's UNIQUE constraint kicked in. Took a frustrated `SELECT count(*)` to spot it.

**`embed.FS` for the SPA is great.** The React frontend ships as static assets inside the Go binary — no separate web server, no nginx, no CORS dance. The Go HTTP mux serves `/api/*` and falls back to `index.html` for everything else, so React Router works without server changes.

**Append-only is one trigger.** That's it:

```sql
CREATE FUNCTION events_immutable() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'events table is append-only';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER events_no_update BEFORE UPDATE ON events
    FOR EACH ROW EXECUTE FUNCTION events_immutable();
CREATE TRIGGER events_no_delete BEFORE DELETE ON events
    FOR EACH ROW EXECUTE FUNCTION events_immutable();
```

Cheap to write, brutal to bypass.

## What's next

The roadmap is in the repo, but the priorities are: **(a)** more data sources — viagens (federal travel), bolsas, pagamentos por programa; **(b)** a public Atom feed of detected drift events for journalists to subscribe to; **(c)** an LAI (Brazilian FOIA) request crawler that watches for unanswered or partially-answered requests across federal agencies.

If you work in the Brazilian transparency space — journalist, prosecutor, civil society dev — and want to plug Fé Pública into something you're building, the API is wide open and I'd love to hear from you.

Code: https://github.com/gmowses/fepublica
Live: https://fepublica.gmowses.cloud
License: AGPL-3.0
