from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from pypimono.engine.infra.mcp.models import (
    RemoteMcpAuthState,
    RemoteMcpOAuthCredentials,
    RemoteMcpPendingLogin,
    RemoteMcpServerConfig,
)
from pypimono.engine.infra.mcp.oauth_client import parse_authorization_input
from pypimono.engine.infra.mcp.token_store import load_remote_mcp_auth_state, save_remote_mcp_auth_state


class RemoteMcpStoreTests(unittest.TestCase):
    def test_auth_state_round_trip(self) -> None:
        server = RemoteMcpServerConfig(
            name="notion",
            mcp_url="https://mcp.notion.com/mcp",
            oauth_resource="https://mcp.notion.com",
            redirect_uri="http://127.0.0.1:14565/oauth/callback",
            client_name="py-pimono",
            scopes=("search:read",),
        )
        state = RemoteMcpAuthState(
            server=server,
            client_id="client-123",
            redirect_uri=server.redirect_uri,
            issuer="https://mcp.notion.com",
            authorization_endpoint="https://mcp.notion.com/authorize",
            token_endpoint="https://mcp.notion.com/token",
            revocation_endpoint="https://mcp.notion.com/token",
            registration_client_uri="https://mcp.notion.com/register/client-123",
            credentials=RemoteMcpOAuthCredentials(
                access_token="access",
                refresh_token="refresh",
                expires_at_ms=1234567890,
                scope="search:read",
            ),
            pending_login=RemoteMcpPendingLogin(
                state="state-123",
                verifier="verifier-123",
                created_at_ms=1000,
                expires_at_ms=2000,
            ),
        )

        with tempfile.TemporaryDirectory() as temp_dir:
            path = Path(temp_dir) / "notion-auth.json"
            save_remote_mcp_auth_state(state, path)
            loaded = load_remote_mcp_auth_state(path)

        self.assertEqual(loaded, state)

    def test_parse_authorization_input_prefers_full_redirect_url(self) -> None:
        code, state = parse_authorization_input(
            "http://127.0.0.1:14565/oauth/callback?code=abc123&state=state-123"
        )

        self.assertEqual(code, "abc123")
        self.assertEqual(state, "state-123")


if __name__ == "__main__":
    unittest.main()
