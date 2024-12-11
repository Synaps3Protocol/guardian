FROM nginx:1.27.3-alpine

COPY default.conf.template /etc/nginx/templates/default.conf.template
# COPY cloudflare.exclusive.conf /etc/nginx/cloudflare.exclusive.conf
CMD ["nginx", "-g", "daemon off;"]