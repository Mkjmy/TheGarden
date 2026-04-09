# the garden

### how to run
1. install docker.
2. run `docker compose up -d`
3. go to `localhost:8080`

### how to host
- run it and it is hosting.
- get onion link: `sudo docker compose exec tor cat /var/lib/tor/hidden_service/hostname`
- give link to others

  
SECURITY NOTE: Default database password is 'change-me'. 
    If you expose port 5432 or modify network settings, change the password in 
    docker-compose.yml immediately to prevent unauthorized access.


### moderation
- to keep the garden alive (avoid read-only): `sudo docker compose exec app python3 mod_tool.py checkin`
- to hide a bad post: `sudo docker compose exec app python3 mod_tool.py hide <id>`
- for more mod commands: `sudo docker compose exec app python3 mod_tool.py help`

### dev status
this garden is still growing. i am still developing more things for it.


