package operations

/*
async function returnWBGL(Chain, conversion, address) {
	try {
	  conversion.status = "returned";
	  await conversion.save();
	  conversion.returnTxid = await Chain.sendWBGL(
		address,
		conversion.amount.toString(),
	  );
	  await conversion.save();
	} catch (e) {
	  console.error(
		`Error returning WBGL (${Chain.id}) to ${address}, conversion ID: ${conversion._id}.`,
		e,
	  );
	  conversion.status = "error";
	  await conversion.save();
	}
  }
*/
