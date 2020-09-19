#!/usr/bin/env bash
set -e

PSQL_HOST=${1:-localhost}
PSQL_DB=${2:-grpc_sample}
PSQL_USER=${3:-service}

echo "load database
     from sqlite://database.db
     into postgresql:///grpc_sample

 with include drop, create tables, create indexes, reset sequences
  set work_mem to '16MB', maintenance_work_mem to '512 MB';
  -- EXCLUDING TABLE NAMES MATCHING 'rating_categories'
  " > loader.conf

pgloader loader.conf

# Fix rating_categories issue cause by decimal value in the integer column
echo "ALTER TABLE rating_categories ALTER COLUMN weight TYPE NUMERIC;
INSERT INTO rating_categories(id,name,weight)
VALUES('1','Spelling','1'),('2','Grammar','0.7'),('3','GDPR','1.2'),('4','Randomness','0')
ON CONFLICT (id)
DO NOTHING;" > fix.sql
psql -h ${PSQL_HOST}  -d ${PSQL_DB} -U ${PSQL_USER} -f fix.sql
