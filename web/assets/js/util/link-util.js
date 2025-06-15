// go 调用

function base64(str) {
    return Base64.encode(str);
}

function genLink(inbound, allSetting) {
    const dbInbound = new DBInbound(inbound)
    const link = dbInbound.genLink(allSetting);
    return link;
}