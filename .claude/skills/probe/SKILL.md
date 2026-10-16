---
# repo-specific: terraform-provider-gns3
name: probe
description: Check how a live GNS3 2.2 server really behaves, on an isolated server started by Invoke-Build Gns3Start, and record the findings in docs/api-drift.md. Use before trusting the API docs for a new endpoint or behaviour.
---

## When to use

Before building on an endpoint whose behaviour the 2.2.52 docs leave unclear, or when a test and the docs disagree.

## Steps

1. `Invoke-Build Gns3Status`. If a server already answers on 3080 and `StartedByBuild` is empty, it is Jerry's (the GUI). Don't probe it. Use another port: `Invoke-Build Gns3Start -Gns3Port 3090`. Otherwise `Invoke-Build Gns3Start`. Its config and data live under `$env:TEMP\gns3`.
2. Use `Invoke-RestMethod` against `http://127.0.0.1:<port>/v2/...`. **Array trap:** pwsh 7 writes a JSON array to the pipeline as one object, so unroll lists: `Invoke-RestMethod "$s/v2/projects" | ForEach-Object { $_ }`.
3. Use throwaway names prefixed `tf-probe-`:
   ```powershell
   $s = 'http://127.0.0.1:3080'
   $p = Invoke-RestMethod "$s/v2/projects" -Method Post -ContentType application/json -Body (@{ name = 'tf-probe' } | ConvertTo-Json)
   $n = Invoke-RestMethod "$s/v2/projects/$($p.project_id)/nodes" -Method Post -ContentType application/json -Body (@{ name = 'PC1'; node_type = 'vpcs'; compute_id = 'local' } | ConvertTo-Json)
   Invoke-RestMethod "$s/v2/projects/$($p.project_id)/nodes/$($n.node_id)/files/startup.vpc" -Method Post -Body "set pcname PC1`n"
   Invoke-RestMethod "$s/v2/projects/$($p.project_id)/nodes/$($n.node_id)/files/startup.vpc"
   Invoke-RestMethod "$s/v2/projects/$($p.project_id)/nodes/$($n.node_id)" -Method Delete
   Invoke-RestMethod "$s/v2/projects/$($p.project_id)/close" -Method Post
   Invoke-RestMethod "$s/v2/projects/$($p.project_id)" -Method Delete
   ```
   Every probe deletes what it created, in that order.
4. `Invoke-Build Gns3Stop`. It stops only the server Gns3Start recorded.
5. Record the finding in `docs/api-drift.md` under the right section: endpoint and docs link, **Docs:**, **2.2.x:** (the version `GET /v2/version` reported, the date and how the server was started), and **Client now:**.

## Stop and ask when

- The probe disagrees with the docs. Record it, then ask before changing the client.
- The only server available is Jerry's GUI server, or a step would touch `%APPDATA%\GNS3` or `Documents\GNS3`. **Never probe Jerry's real GNS3 data directory.**
- A probe would leave something behind that the API can't remove (images can't be deleted in 2.2).

## Report

Use the report-template skill. Put the server version, the port and each finding (with its `api-drift.md` heading) under Summary.
