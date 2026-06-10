FROM mcr.microsoft.com/mssql/server:2022-latest
USER root
RUN apt-get update -q \
    && apt-get install -y mssql-server-fts \
    && rm -rf /var/lib/apt/lists/*
USER mssql
