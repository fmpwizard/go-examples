define(function (require){
  'use strict';
  var defineComponent = require('flight/lib/component');
  return defineComponent(AddMessage);
  function AddMessage() {

    this.defaultAttrs ({
      messageInput : '#message'
    });

    this.handleSubmmit = function(event){
      event.preventDefault();
      var $message = this.select('messageInput');
      var message = {
        body: $message.val(),
        createdOn: new Date().getTime()
      };

      this.trigger('uiMessageSent', {
        message: message
      });
    };

    this.handleMessageSabed = function (_, data) {
      this.select('messageInput').val('');
    };

    this.after('initialize', function(){
      this.on('submit', this.handleSubmmit);
      this.on(document, 'dataMessageSaved', this.handleMessageSabed);
    });
  }
});