using System.Collections;
using System.Collections.Generic;
using UnityEngine;

public class Obstacle : MonoBehaviour
{
    Transform player;
    public float loseOffset = 2;
    bool done = false;
    public bool isLeft;
    public bool isWall = false;
    private void Start()
    {
        foreach (PlayerController x in FindObjectsOfType<PlayerController>())
        {
            if(x.altLeft == isLeft)
            {
                player = x.transform;
                break;
            }
        }
    }

    private void Update()
    {
        if(transform.position.z - loseOffset > player.position.z)
        {
            if (!done)
                Lose();
            Destroy(gameObject);
        }
    }

    void Lose()
    {
        FindObjectOfType<Server>().Lose(isLeft);
    }

    private void OnTriggerEnter(Collider other)
    {
        if (other.tag == "Player")
        {
            if (!done)
            {
                done = true;
                if(isWall)
                    other.GetComponent<PlayerController>().touchedWall = true;
            }
        }
    }
}
