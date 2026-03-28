FROM migrate/migrate:latest

COPY migrations /migrations

ENTRYPOINT ["migrate", "-path", "/migrations", "-database"]
CMD ["${DATABASE_URL}?search_path=deploy", "up"]