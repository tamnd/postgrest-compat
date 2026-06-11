FROM mcr.microsoft.com/mssql/server:2022-latest
USER root
# Detect Ubuntu release, add the matching Microsoft mssql-server-2022 repo,
# then install the Full-Text Search feature package.
RUN . /etc/os-release \
    && curl -fsSL https://packages.microsoft.com/keys/microsoft.asc \
       | gpg --dearmor -o /usr/share/keyrings/microsoft-prod.gpg \
    && echo "deb [arch=amd64 signed-by=/usr/share/keyrings/microsoft-prod.gpg] \
       https://packages.microsoft.com/ubuntu/${VERSION_ID}/mssql-server-2022 ${VERSION_CODENAME} main" \
       > /etc/apt/sources.list.d/mssql-server-2022.list \
    && apt-get update -q \
    && apt-get install -y --no-install-recommends mssql-server-fts \
    && rm -rf /var/lib/apt/lists/*
USER mssql
