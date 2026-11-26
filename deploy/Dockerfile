# Image d'execution minimale (distroless : ni shell ni gestionnaire de paquets), construite dans le
# dossier de prod a partir du binaire produit par Dockerfile.build. Textes et images sont dedans.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --chmod=0755 proximas-web /proximas-web
USER 65532:65532
ENV LISTEN_ADDR=:8080
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 CMD ["/proximas-web", "healthcheck"]
ENTRYPOINT ["/proximas-web", "serve"]
