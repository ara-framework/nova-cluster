# Nova Cluster
Nova Cluster helps to request views from different hypernova servers using only one endpoint.

## Views File

Nova Cluster needs a configuration file  `views.json` in order to map the views and their hypernova servers. 

```json
{
  "Navbar": {
    "server": "http://localhost:3031/batch"
  },
  "Home": {
    "server": "http://localhost:3030/batch"
  }
}
```

## Using Hypernova Proxy with Docker

```Dockerfile
FROM marconi1992/nova-cluster:1.0.0

COPY views.json views.json
```