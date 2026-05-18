using System.Net;
using System.Net.Sockets;
using System.Text;
using UnityEngine;

public class SimpleServer : MonoBehaviour
{
    UdpClient udpServer;
    IPEndPoint remoteEP, lewy, prawy; // Tu zapisze siê adres klienta, który "zapuka"
    bool czy = false;

    void Start()
    {
        udpServer = new UdpClient(7777); // Nas³uchujemy na porcie 7777
        remoteEP = new IPEndPoint(IPAddress.Any, 0);
        Debug.Log("Serwer wystartowa³...");
    }

    void Update()
    {
        // Sprawdzamy czy przysz³y jakieœ dane (nieblokuj¹ce)
        if (udpServer.Available > 0)
        {
            byte[] data = udpServer.Receive(ref remoteEP);
            string input = Encoding.ASCII.GetString(data);

            if (!czy) lewy = remoteEP;
            else if(remoteEP != lewy) prawy = remoteEP;
            czy = true;

            Debug.Log($"Odebrano input: {input} od {remoteEP}");

            if (input != "N")
            {
                if (remoteEP == lewy)
                {

                }
                else
                {

                }
            }
            // Logika: Przeliczamy pozycjê na podstawie inputu
            Vector3 newPosition = HandleInputAndCalculatePosition(input);

            // Wysy³amy odpowiedŸ do tego samego klienta
            string response = $"{newPosition.x};{newPosition.y};{newPosition.z}";
            byte[] responseData = Encoding.ASCII.GetBytes(response);
            udpServer.Send(responseData, responseData.Length, remoteEP);
        }
    }

    Vector3 HandleInputAndCalculatePosition(string input)
    {
        // Tutaj Twoja logika ruchu...
        return new Vector3(1, 0, 5);
    }
}