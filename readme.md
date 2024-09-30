# RESTful Ethereum Validator API
## Objective
Develop a small RESTful API application using Go. This application will interact with Ethereum data to provide information about block rewards and validator sync committee duties per slot.

## Design
In general, this api server is built to fetch data while request from client calling in. It is set with `httprate.LimitByIP` 100 reqs/min to avoid api exploit since it is query from quicknode api. We have to pay for the credit for each client request. It can be extended later with client api key allocated requested limit with each individual client.

- `blockreward` 
It is fetching all the relays endpoints to find the bidtraces of block. If if reqested slot exists from the relay's bidtraces, the last transaction of the block (to address == bidtraces'proposer_fee_recipient) and eth transfer value == bidtraces.value, it deteminate it is a block produced by a MEV relay. The `blockreward` should be `bidtraces.value`

When no bidtraces is found matching, it is a a vanilla block (built internally in the validator node). The `blockreward` should be `aggregating gas_used*gas_price for every transaction in that block and then deducting the burned fee`. `static reward` is ignored here since after the merge to PoS. No more miner reward. Assuming we only want the latest data. Otherwise, I could also add back the static reward to calculation.

- `syncduties` 
It is fetching `/eth/v1/beacon/states/{slot}/sync_committees` to get indexes of all validators which are sync committees. And then it needs to look up `/eth/v1/beacon/states/{slot}/validators` to map the indexes to the validator pubkey. By my observation, `/eth/v1/beacon/states/{slot}/validators` has around 750MB for the latest block. It is not worth it to call this endpoints for every single client request, since sync committees are chosen every 256 epochs (~27 hours). The need response of `/eth/v1/beacon/states/{slot}/validators` is store as a index-pubkey map{key: index, value: pubkey}. When the application starts, it calls `/eth/v1/beacon/states/{slot}/validators` to load up the map in memory. When api call from client to our `syncduties` api, it looks ip the map to find the pubkey. If there is an unknown indexes (new validator just joined and selected to the network), our app call `/eth/v1/beacon/states/{slot}/validators` in singleflight to update the index-pubkey map again.

### package
- go 1.23.1
- go-chi
go-chi router is compatible with go built-in library net/http. It is light-weighted and provide various support of existing middleware
- log/slog
light-weighted logger
- github.com/namsral/flag
flag

### middleware
- logger
logger is injected to middleware to log every requests frim client


## How to build and run your application
1. make a copy of `example.env` to `.env`
```bash
cp example.env .env
```
2. add your quicknode http endpoint key in .env - param `NODE_ENDPOINT`
3. make sure is app is running in port 8080. If so, change param `LISTEN_ADDR`. And test it with your own port.
4. We can run it in 2 way
- script
```bash
chomd +x run.sh
./run.sh
```
- docker
```bash
docker compose up && docker compose rm -f
```

## Examples of how to call the API endpoints
### GET /blockreward/{slot}
```bash
curl http://localhost:8080/eth/validator/blockreward/{slot}
```

Example response:

curl http://localhost:8080/eth/validator/blockreward/10074111
```JSON
{
    "status":"mev",
    "reward":"67088815589653475"
}
```

curl curl http://localhost:8080/eth/validator/blockreward/10073863
```JSON
{
    "status":"vanilla",
    "reward":"11958365682073055"
}
```

curl http://localhost:8080/eth/validator/blockreward/abcd
```
Not Found
```

curl http://localhost:8080/eth/validator/blockreward/20074111
```
Bad Request
```

### GET /syncduties/{slot}
```bash
curl http://localhost:8080/eth/validator/syncduties/{slot}
```
Please use the latest slot(https://beaconcha.in) to have a faster response speed. If an old slot is used with this app, it can take longer to have response from quicknode.

Example response:

curl http://localhost:8080/eth/validator/syncduties/10074637
```
{
    "data":[
        "0xb9cc8496bba9566b11e12dceffd2b0bc235d78bd457c5f45856b772f949d7ddfa5ea0eac40eb3dd594dd03df2728ba62","0x96d18e1f7e84b2895d1fb42ae8d25c90f9eb38895af13b72b6416ea386373ba932a69a5ee7469f5221d1fd1f58e66123",
        ...
    ]
}
```

curl http://localhost:8080/eth/validator/syncduties/abcd
```
Not Found
```

curl http://localhost:8080/eth/validator/syncduties/20074111
```
Bad Request
```

## How to run test
```bash
chomd +x test.sh
./test.sh
```

## Test
1. Integration tests
- testing/integrationtests/server_GetBlockreward_test.go
- testing/integrationtests/server_GetSyncduties_test.go
2. Unit tests
- pkg/ethnode/beacon_test.go
- pkg/ethnode/rpc_client_test.go
- pkg/relay/relay_test.go

## Solution Logic Detailed Explain
### GET /blockreward/{slot}
1. check if slot is unit64
2. check if slot is in future
3. call all relays {relay endpoint}/relay/v1/data/bidtraces/proposer_payload_delivered?limit=1&cursor={slot}
- check if reqested slot exists from the relays'bidtraces
- if slot exist -> "value" from bidtraces is validator reward
4. check if last txs in block recipient == bidtraces'proposer_fee_recipient && eth transfer == bidtraces.value
5. (edge case)if block feeRecipient == proposer_fee_recipient, reward = bidtraces.value + (minerReward + aggregating gas_used*gas_price for every transaction in that block and then deducting the burned fee)
6. if it is not built with relay, validator takes the block reward (minerReward + aggregating gas_used*gas_price for every transaction in that block and then deducting the burned fee)

Relay data source
path: "/relay/v1/data/bidtraces/proposer_payload_delivered?limit={}&cursor={}"

from the last 180days:
https://beaconcha.in/relays#t180d

from article:
https://github.com/nerolation/mevboost.pics/blob/main/scripts/parse_data_api.py#L67-L77

List from relays used in this server:
```
https://boost-relay.flashbots.net
https://bloxroute.max-profit.blxrbdn.com
https://bloxroute.regulated.blxrbdn.com 
https://mainnet-relay.securerpc.com 
https://relay.edennetwork.io 
https://relay.ultrasound.money
https://agnostic-relay.net https://aestus.live
https://mainnet.aestus.live https://titanrelay.xyz
https://mainnet-relay.securerpc.com
https://relay.wenmerge.com
https://mainnet-relay.securerpc.com
https://regional.titanrelay.xyz
https://global.titanrelay.xyz
https://relay.edennetwork.io
```

### GET /syncduties/{slot}
(assuming validator index won't change)
1. app start with call {node http endpoint}/eth/v1/beacon/states/{latest_slot}/validators
- load all the validators for requested slot to a map[index]pubkey in memory
2. check if slot is unit64
3. check if slot is in future
4. call {node http endpoint}/eth/v1/beacon/states/{slot}/sync_committee
- retrive all the indexes for the request slot
5. look up all the pubkey from the map with indexes from step 1
- if not exist, lazy load singleflight update the map (by step 1 call {node http endpoint}/eth/v1/beacon/states/{latest_slot}/validators)

## improvement
In general, if historical data is needed, I would make it as 2 app - exporter and api-server. Exporter computes the data concurrenily and stores the data in database. Api-server will fetch the data from database.

### blockreward
- It has more edge cases that needs to be covered. I need more time to investigate, if the builder has different setting with the block building
- Relay endpoints has to be updated from time to time, when new relay is up for the network. It needs another relay endpoint scrapper to keep the data is up to date

### syncduties
- very huge response from: {node http endpoint}/eth/v1/beacon/states/{slot}/validators
It would be better to have another go routine or exporter to keep the indexes and pubkey in database to look up
- very slow with old slot: {node http endpoint}/eth/v1/beacon/states/{slot}/sync_committees
It could only provide latest slot with fast response for current set-up. It would be better to store every historical record and computed in the back for the api server.
