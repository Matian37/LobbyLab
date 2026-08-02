FROM node:26-alpine

WORKDIR /api

COPY package*.json ./
RUN npm ci

COPY . .

RUN npm run build

EXPOSE 3000

CMD ["node", "server.js"]