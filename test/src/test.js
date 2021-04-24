const ansi = require('ansi-colors');
// Without a transform you would see this text in yellow.
export default (message) => ansi.bold.yellow(message)