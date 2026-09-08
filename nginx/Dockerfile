FROM nginx:1.27-alpine

# デフォルトの default.conf を自前の設定で置き換える（:80 の重複を避ける）
COPY conf.d/app.conf /etc/nginx/conf.d/default.conf
