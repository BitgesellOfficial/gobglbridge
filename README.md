# BGL-WBGL golang bridge implementation

Bridge service arhitecture:

1. Block scanner Eth / Bsc / other EVM networks
   - check incoming transfers and put sending to worker queue;
2. Block scanner BGL
   - check incoming transfers and put sending WBGL to worker queue;
3. Sending worker BGL / EVM
4. Manage address
5. TODO: Dashboard / Display status (by address or tx id)

Data structures:
1. address book: mapping BGL -> EVM WBGL, EVM BGL -> BGL and vice versa;

2. transaction history, status
   - pending (receiving tx detected)
   - sent (output tx sent)
   - success (output tx processed)

   - returned no funds output chain
   - returned no funds for gas output chain
   - unknown route (don't know destination to match sender address to)
   - fail (technical error occured, fail to return, etc.)

as resources are super-constrained (so no infrastructure expenses), should be
runnable on something like micro instance (1vCPU/512Mb RAM);

- monolithic, no docker, no docker-compose golang service;
    - statically served HTML app (by the same binary);
    - API service;
    - block scanners and tx sending workers;
- BGL node (to have reliable RPC, probably remote could be used if needed);
- Redis (as compact local persistence engine);
    - no SQL queries, simple key-value
- round-robin on EVM RPCs (to use freely available);

- compatible by request/response to keep existing bridge webapp working with mininal changes;
