## Run Casdoor
`docker run -p 8000:8000 casbin/casdoor-all-in-one`

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
