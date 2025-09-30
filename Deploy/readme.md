```dockerfile
docker run --rm -it \
                                                                                          --network=host \
                                                                                          -e driverName=postgres \
                                                                                          -v ./init_data.json/:/init_data.json \
                                                                                          -e dataSourceName='user=postgres password=postgres host=127.0.0.1 port=5432 sslmode=disable dbname=casdoor-test' \
                                                                                          casbin/casdoor:latest
```