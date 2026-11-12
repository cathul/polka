# Polka — cathul fork

Self-hosted ebook library with a web reader, OPDS, KOReader progress synchronization
and email delivery. Release **0.1**, based on
[vestigiumincaligne/polka v0.1.22](https://github.com/vestigiumincaligne/polka/tree/v0.1.22).
English and Russian interface.

## Install with Docker

Requires Docker Engine and Docker Compose v2 or later on Linux amd64 or arm64.
Download `compose.yaml` and `deploy.env.example` from the
[release](https://github.com/cathul/polka/releases/tag/cathul-v0.1) into an empty directory.
Copy `deploy.env.example` to `.env` and edit:

```dotenv
POLKA_BOOKS_PATH=/absolute/path/to/books
POLKA_BIND_IP=127.0.0.1
POLKA_BIND_PORT=12791
```

The books directory must already exist and be readable by the container user.
It is mounted read-only at `/books`. Use `POLKA_BIND_IP=0.0.0.0` for LAN access;
then open `http://SERVER_IP:12791`. For local access use `http://127.0.0.1:12791`.
A reverse proxy or Cloudflare Tunnel can point to the same HTTP service. Set the
public address in Polka if your deployment uses a fixed external URL.

```sh
docker compose pull
docker compose up -d
docker compose logs polka
```

The initial administrator credentials appear in the logs. Import your INPX in
Administration using its **container path**, for example `/books/library.inpx`.
Accounts, settings, catalog and progress persist in the named data volume.
SMTP and conversion are configured by the administrator in Polka. Each user sets
their own email recipient from the account menu. No extra converter container,
Go installation or GitHub login is needed to run the public image.

## Changes in this fork

- FB2 → EPUB3 converter for email delivery; one administrator SMTP account,
  personal recipient addresses, author/title filenames and localized messages.
- Personal **Want to read** OPDS shelf, using the existing web wishlist.
- Independent single-language web search and personal OPDS search preference;
  matching books without a language are always included.
- Safe INPX replacement: stable book identities and preserved digest mappings,
  reading progress and lists. Last successful INPX server path is remembered.
- Accurate two-way FB2 position synchronization with KOReader. EPUB retains
  approximate synchronization; the FB2 position bridge does not apply to EPUB.
- Personal progress removal and refreshed book actions, header, account menu
  and footer. New interface elements support EN/RU.

See [release notes](docs/releases/0.1.md).
The [upstream documentation](docs/UPSTREAM_README.md) describes the original project;
its download links and Docker image refer to upstream, not this fork.

## Updates and existing installations

Before migrating, stop the existing Polka and back up its **entire data directory**,
including SQLite files and encryption keys. Test the fork on a copy first. Never
point two running instances at the same data directory. Restore the full backup
and the previous image to roll back; do not downgrade an already migrated database.
The converter sidecar and Cloudflare routing from an older custom installation
must be changed separately when switching production traffic to this service.

For an existing deployment, keep its volume and configuration and change its
image to `ghcr.io/cathul/polka:latest`. The supplied Compose file creates an independent
`polka-fork` project; it does not automatically adopt existing data.

The supplied Compose file uses `:latest`. To download and apply the next stable release:

```sh
docker compose pull
docker compose up -d
```

Keep the data volume; **do not use `down -v`**. `:latest` tracks stable releases;
`:0.1` selects this release. Release source is tagged `cathul-v0.1`.

## Development and publication

- `main` tracks original upstream; `develop` contains this fork and is the default branch.
- Fork version is stored in `VERSION`, independently of upstream numbering.
- Develop CI runs frontend/Go tests, vet and a two-platform Docker build.
- Run **Actions → Fork release → Run workflow** on `develop` to create the release
  tag and publish. Existing tags must point to the same commit.
- Tags `cathul-v*` also run verification, publish to `ghcr.io/cathul/polka` and create
  a GitHub Release with installation files. The tag version must match `VERSION`.
- `docker-compose.develop.yml` and `.env.example` provide a separate local trial.
  Use `docker compose -f docker-compose.develop.yml ...` explicitly. The original
  `docker-compose.yml` remains available explicitly for upstream.
- Local `.env`, old server files, test data and build tools are excluded from Git.

The image bundles the pinned [fb2cng](https://github.com/rupor-github/fb2cng)
converter as a separate executable, including its license and matching source.
See [converter source information](build/CONVERTER-SOURCE.txt). Original Polka
copyright and [license](LICENSE) are retained.
