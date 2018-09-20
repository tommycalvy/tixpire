
// JavaScript
window.sr = ScrollReveal();

// sr.reveal('h1', {
//     delay: 0,
//     duration: 200,
//     origin: 'bottom',
//     distance: '100px'
// });
function showNav() {
  var x = document.getElementById("responsive-nav");
  if (x.className === "responsive-nav") {
    x.className += " unfold";
  } else {
    x.className = "responsive-nav";
  }
}

function signupOrLoginPopUp() {
  var x = document.getElementById("popup-box1");
  if (x.className === "popup-position") {
    x.className += " open";
  } else {
    x.className = "popup-position";
  }

}
