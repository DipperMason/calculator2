CREATE TABLE expressions (
    id INTEGER PRIMARY KEY,
    expression TEXT,
    responses TEXT,
    user TEXT
);

CREATE TABLE id_counter (
    id INTEGER
);
INSERT INTO id_counter (id) VALUES (0);