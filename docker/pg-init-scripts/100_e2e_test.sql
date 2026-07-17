INSERT INTO oauth_clients (id, name, redirect_uri) 
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'e2e test client',
    'https://client.example/callback'
)
ON CONFLICT (id) DO NOTHING;
