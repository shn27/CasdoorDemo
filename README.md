# CasdoorDemo

------------

## Run Postgres

```dockerfile
 docker run --name casdoor-postgres -d \
                        -p 5432:5432 \
                        -e POSTGRES_USER=postgres \
                        -e POSTGRES_PASSWORD=postgres \
                        -e POSTGRES_DB=casdoor-test \
                        postgres:15-alpine
```

## Run Casdoor
This will create an org, an app, a cert, some providers and a user under the app.
```
docker run --rm -it \
--network=host \
-e driverName=postgres \
-v ./Deploy/init_data.json/:/init_data.json \
-e dataSourceName='user=postgres password=postgres host=127.0.0.1 port=5432 sslmode=disable dbname=casdoor-test' \
casbin/casdoor:latest
```

### Configuration Setup
#### Create an Organization
```azure
In the UI, navigate to Organizations → Add.

Example:

Organization name: my-org

Display name: My Organization

This my-org will later be used as the owner.
```
#### Create an Application
```azure

Go to Applications → Add.

Fill in:

Application name: my-app

Organization: my-org

Choose a login type (e.g., Username/Password, Google, GitHub, etc.)

Set Redirect URL (e.g., http://localhost:7000/callback if you’re just testing; it must match your client app’s redirect endpoint).
```
#### Add New User
```
curl --location 'http://localhost:8000/api/add-user' \
--header 'Authorization: Bearer token \
--header 'Content-Type: application/json' \
--header 'Cookie: casdoor_session_id=728a572868f9ddba05dd200c4fc2ddd0' \
--data-raw '{
        "owner": "my-org",
        "name": "alice",
        "displayName": "Alice",
        "password": "123",
        "email": "alice@example.com"
      }'
```
#### set env according to my-app
```azure
CASDOOR_ENDPOINT 
CASDOOR_CLIENT_ID 
CASDOOR_CLIENT_SECRET 
CASDOOR_ORGANIZATION 
CASDOOR_APPLICATION 
CASDOOR_REDIRECT_URI
CASDOOR_CERTIFICATE
```
##### use this scirpt
```bash
for line in (cat .env)
    set -l key (echo $line | cut -d '=' -f1)
    set -l val (echo $line | cut -d '=' -f2- | sed 's/^"//;s/"$//')
    set -Ux $key $val
end
```
`set -Ux CASDOOR_CERTIFICATE "<CERTICICATE>""`

------
### SignIn Using Github Steps
##### Github OAuth EASY credentials setup
Add a new OAuth App
- https://github.com/settings/developers
    - Homepage URL : http://localhost:8000
    - Authorization callback URL : http://localhost:8000/callback
     
- Help: https://www.youtube.com/watch?v=R9lxXQcy-nM

##### Add a provider using this json where clientId and clientSecret should be from `https://github.com/settings/applications/<APPLICATIONID>`

```json
    "providers": [
    {
    "owner": "admin",
    "name": "provider-github",
    "createdTime": "",
    "displayName": "GitHub OAuth Provider",
    "category": "OAuth",
    "type": "GitHub",
    "subType": "",
    "method": "",
    "clientId": "GITHUB_CLIENT_ID_https://github.com/settings/applications",
    "clientSecret": "GITHUB_CLIENT_SECRET_https://github.com/settings/applications",
    "clientId2": "",
    "clientSecret2": "",
    "cert": "",
    "customAuthUrl": "",
    "customTokenUrl": "",
    "customUserInfoUrl": "",
    "customLogo": "",
    "scopes": "user:email read:user",
    "userMapping": {},
    "host": "",
    "port": 0,
    "disableSsl": false,
    "title": "GitHub",
    "content": "",
    "receiver": "",
    "regionId": "",
    "signName": "",
    "templateCode": "",
    "appId": "",
    "endpoint": "",
    "intranetEndpoint": "",
    "domain": "",
    "bucket": "",
    "pathPrefix": "",
    "metadata": "",
    "idP": "",
    "issuerUrl": "",
    "enableSignAuthnRequest": false,
    "providerUrl": ""
    }]
```

------
### SignIn Using Google Steps
##### Google OAuth EASY credentials setup
- https://console.cloud.google.com/auth/branding?organizationId=0&project=casdoordemo
    - App domain section should be empty for localhost

- https://console.cloud.google.com/auth/clients/147761870281-ocrpjauf5hs2m4u8tsa2lqskk75f3abu.apps.googleusercontent.com?organizationId=0&project=casdoordemo
    - Authorised JavaScript origins : http://localhost:8000
    - Authorised redirect URIs : http://localhost:8000/callback

- Help: https://www.youtube.com/watch?v=TjMhPr59qn4

##### Add a provider using this json where clientId and clientSecret should be from `Google Cloud Console`
```json
"providers": [
    {
      "owner": "admin",
      "name": "provider-google",
      "createdTime": "",
      "displayName": "Google OAuth Provider",
      "category": "OAuth",
      "type": "Google",
      "subType": "",
      "method": "",
      "clientId": "CLIENT_ID_FROM_GOOGLE_CLOUD_CONSOLE",
      "clientSecret": "CLIENT_SECRET_FROM_GOOGLE_CLOUD_CONSOLE",
      "clientId2": "",
      "clientSecret2": "",
      "cert": "",
      "customAuthUrl": "",
      "customTokenUrl": "",
      "customUserInfoUrl": "",
      "customLogo": "",
      "scopes": "profile email",
      "userMapping": {},
      "host": "",
      "port": 0,
      "disableSsl": false,
      "title": "Google",
      "content": "",
      "receiver": "",
      "regionId": "",
      "signName": "",
      "templateCode": "",
      "appId": "",
      "endpoint": "",
      "intranetEndpoint": "",
      "domain": "",
      "bucket": "",
      "pathPrefix": "",
      "metadata": "",
      "idP": "",
      "issuerUrl": "",
      "enableSignAuthnRequest": false,
      "providerUrl": ""
    }]
```
------
#### Email verification setup while SignUp 
- Add new App Password in https://myaccount.google.com/apppasswords
- Add a provider 
```
{ 
  Name : email-provider,
  Displayname : Email Provider,
  Organization : admin(Shared),
  Category : Email,
  Type : Default,
  Username : sohanur1525sohan@gmail.com,
  Password : <PASS_FROM_https://myaccount.google.com/apppasswords>
  From address : sohanur1525sohan@gmail.com,
  From name : no-reply,
  Host : smtp.gmail.com, 
  Port : 587,
  Disable SSL : true,
  Email title : Casddor Verification Code, 
}
```
- We will use zepto rather smtp.gmail.com
```
[mailer]
ENABLED             = true
SUBJECT_PREFIX      = bb.test |
HOST                = smtp.zeptomail.com:587
IS_TLS_ENABLED      = false
FROM                = no-reply@appscode.com
USER                = emailapikey
PASSWD              = 
SEND_AS_PLAIN_TEXT  = false
MAILER_TYPE         = smtp
```
- Help: https://www.youtube.com/watch?v=H0HZc4FgX7E


##### Add a provider using this json where Password should be from `https://myaccount.google.com/apppasswords`
- https://myaccount.google.com/apppasswords?continue=https://myaccount.google.com/security?gar%3DWzEyMF0%26hl%3Den_GB%26utm_source%3DOGB%26utm_medium%3Dact&pli=1&rapt=AEjHL4PLjHmX06m3ZgiROXKZPKg8PTS4RPUUU-JwrghqNmW3xQGR940e9V75yQbEV3rU16mkhJnVcjepP_fnYVaJMqXMaYy67d9QB7GKdbgobsoBjsgqtRk

```json
"providers": [
    {
    "owner": "admin",
    "name": "provider-email",
    "createdTime": "",
    "displayName": "Email Provider",
    "category": "Email",
    "type": "Default",
    "method": "Normal",
    "clientId": "sohanur1525sohan@gmail.com",
    "clientSecret": "PASS_FROM_https://myaccount.google.com/apppasswords",
    "clientId2": "sohanur1525sohan@gmail.com",
    "clientSecret2": "no-reply",
    "cert": "",
    "customAuthUrl": "",
    "customTokenUrl": "",
    "customUserInfoUrl": "",
    "customLogo": "",
    "scopes": "",
    "userMapping": {},
    "host": "smtp.gmail.com",
    "port": 587,
    "disableSsl": true,
    "receiver": "sohan@appscode.com",
    "regionId": "",
    "signName": "",
    "templateCode": "",
    "appId": "",
    "endpoint": "",
    "intranetEndpoint": "",
    "domain": "",
    "bucket": "",
    "pathPrefix": "",
    "metadata": "",
    "idP": "",
    "issuerUrl": "",
    "enableSignAuthnRequest": false,
    "providerUrl": ""
    "title": "Casdoor Verification Code"
    } ]
```