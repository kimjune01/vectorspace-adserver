api.${DOMAIN} {
	reverse_proxy server:8080
}

portal.${DOMAIN} {
	reverse_proxy server:8080
}
