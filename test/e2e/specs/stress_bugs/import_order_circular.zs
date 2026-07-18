// BUG (wiki "Import order triggers a false circular dependency"): importing the leaf BEFORE
// the intermediate that also imports it is wrongly flagged as a cycle. Swapping the two
// import lines compiles fine. When fixed, this order also compiles.
import { leaf } from "./ximport_leaf";
import { mid } from "./ximport_mid";
function main(): i32 { return leaf() + mid() - 3; }
