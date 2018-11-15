
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

function calendarApp() {
  window.eventCalId=3308;
  var integrationScript = document.createElement("script");
  integrationScript.async = 1;
  integrationScript.setAttribute("src", "https://api.eventcalendarapp.com/integration-script.js");
  document.head.appendChild(integrationScript);
  if (window.eventCalendarAppUtilities) {
    window.eventCalendarAppUtilities.init("0a94d5e1-6d05-4ee6-a962-24471e23ed95");
  }
}
