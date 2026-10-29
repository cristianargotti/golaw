# valkey/valkey:8.1
FROM valkey/valkey@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
COPY valkey.conf /etc/valkey/valkey.conf
CMD ["valkey-server", "/etc/valkey/valkey.conf"]
