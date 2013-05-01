goftp
=========

forked from dennisfrancis/goftp

What I've done
--------------

Make it compatible to FTP server like Serv-U FTP Server v4.0 for WinSock. 
The response are followed disk information after excuting LIST command:

	226-Maximum disk quota limited to 1000000 Kbytes
	    Used disk quota 0 Kbytes, available 1000000 Kbytes
	226 Transfer complete.


HOW TO
------
I wrote a MyReadCodeLine function to replace the ReadCodeLine() in package 
net/textproto.

Some changes are made:

1. ingnore the "unexpected multi-line response" err
2. for response "226-Maximum disk quota limited to 1000000 Kbytes",
   just readLine() two more times.
   
TODO
------
- Deal with the welcome message.

It's still in development, so there are some commentted fmt sentences for debug.
