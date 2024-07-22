package operations

/*
async function returnBGL(conversion, address) {
  try {
    conversion.status = "returned";
    await conversion.save();
    conversion.returnTxid = await RPC.send(address, conversion.amount);
    await conversion.save();
  } catch (e) {
    console.error(
      `Error returning BGL to ${address}, conversion ID: ${conversion._id}.`,
      e,
    );
    conversion.status = "error";
    await conversion.save();
  }
}
*/
