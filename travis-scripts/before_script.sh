#!/bin/bash
set -ev
if [ "$BACKEND" = 1 ]; then
    mysql -e 'create database travis_ci;'
    mv backend/api/.env.travis backend/api/.env
    python backend/manage.py migrate
fi

if [ "$FRONTEND" = 1 ]; then
    mv frontend/.env.travis frontend/.env
fi

if [ "$TESTS" = "e2e" ]; then
    npm i -g cypress
    cd backend && sh load_fixtures.sh && cd ..
    python backend/manage.py runserver 8000 &
    wget http://localhost:8000/api/auth/login
    cd frontend && npm start -- --silent &
fi