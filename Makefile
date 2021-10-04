myxds_api:
	protoc --go_out=./ --go-grpc_out=./ ./myxds/api/bootstrap.proto

jsonserver:
	deno run --allow-read --allow-write --allow-net ./example/json-server.ts ./example/json-server-db.json --port 8853
