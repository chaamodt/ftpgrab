# HTTP(S) server configuration

!!! warning
    `ftp`,`sftp` and `http` are mutually exclusive

!!! example
    ```yaml
    server:
      http:
        host: 10.0.0.1
        port: 22
        username: foo
        password: bar
        sources:
          - /
        timeout: 30s
        tls: true
        insecureSkipVerify: false
    ```

## Reference

### `host`

HTTP host IP or domain.

!!! example "Config file"
    ```yaml
    server:
      http:
        host: 127.0.0.1
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_HTTP_HOST`

### `port`

HTTP port. (default `80`)

!!! example "Config file"
    ```yaml
    server:
      http:
        port: 80
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_HTTP_PORT`

### `username`

HTTP username for basic auth.

!!! example "Config file"
    ```yaml
    server:
      http:
        username: foo
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_HTTP_USERNAME`

### `usernameFile`

Use content of secret file as HTTP username if `username` not defined.

!!! example "Config file"
    ```yaml
    server:
      http:
        usernameFile: /run/secrets/username
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_HTTP_USERNAMEFILE`

### `password`

!!! warning
    `password` and `keyFile` are mutually exclusive

HTTP password for basic auth.

!!! example "Config file"
    ```yaml
    server:
      http:
        password: bar
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_HTTP_PASSWORD`

### `passwordFile`

Use content of secret file as HTTP password if `password` not defined.

!!! example "Config file"
    ```yaml
    server:
      http:
        passwordFile: /run/secrets/password
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_HTTP_PASSWORDFILE`

### `sources`

List of sources paths to grab from HTTP server. Must be url to resources as filename is used to save the resource.

!!! example "Config file"
    ```yaml
    server:
      http:
        sources:
          - /path1/file.txt
          - /path2/folder/file.txt
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_HTTP_SOURCES`

### `userAgent`

Optional `User-Agent` header for HTTP requests. Set this when the remote server requires a particular client identifier. If omitted, Go's default User-Agent is used.

!!! example "Config file"
    ```yaml
    server:
      http:
        userAgent: Wget
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_HTTP_USERAGENT`

### `timeout`

Timeout is the maximum amount of time for the TLS Handshake and the maximum amount of time to wait for a server's response headers after fully writing the request. (default `5s`)

!!! example "Config file"
    ```yaml
    server:
      http:
        timeout: 5s
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_HTTP_TIMEOUT`

### `DisableKeepAlives`

If true disables HTTP keep-alives and will only use the connection to the server for a single HTTP request. (default false)

!!! example "Config file"
    ```yaml
    server:
      http:
        DisableKeepAlives: false
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_HTTP_DISABLEKEEPALIVES`

### `DisableCompression`

 If true, prevents the Transport from requesting compression with an "Accept-Encoding: gzip" request header when the Request contains no existing Accept-Encoding value. If the Transport requests gzip on its own and gets a gzipped response, it's transparently decoded in the Response.Body. However, if the user explicitly requested gzip it is not automatically uncompressed. (default false)

!!! example "Config file"
    ```yaml
    server:
      http:
        DisableCompression: false
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_HTTP_DISABLECOMPRESSION`

### `tls`

Use TLS (https). (default `false`)

!!! example "Config file"
    ```yaml
    server:
      http:
        tls: false
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_HTTP_TLS`

### `insecureSkipVerify`

Controls whether a client verifies the server’s certificate chain and host name. (default `false`)

!!! example "Config file"
    ```yaml
    server:
      http:
        insecureSkipVerify: false
    ```

!!! abstract "Environment variables"
    * `FTPGRAB_SERVER_HTTP_INSECURESKIPVERIFY`

