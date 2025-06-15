
function base64(str) {
    return Base64.encode(str);
}

function genLink(inbound) {
    const dbInbound = new DBInbound(inbound)
    const link = dbInbound.genLink();
    return link;
}