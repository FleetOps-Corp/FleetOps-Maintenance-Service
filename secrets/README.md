# Secrets Directory (Local Development Only)

This directory contains key material used **exclusively for local development and unit/integration testing**.

## Files
- `jwt_public.pem`: A development-only mock RSA public key.
- `jwt_private.pem`: A development-only mock RSA private key (untracked and ignored by Git).

> [!CAUTION]
> **Production Key Management**:
> Never commit actual production or staging keys to this or any source code repository. In production environments, the public key should be securely injected from outside the repository using secret managers (e.g. AWS Secrets Manager, HashiCorp Vault) or orchestrated container secret mounts (e.g. Kubernetes Secrets, Docker Compose bind mounts from a secure out-of-repo directory).
